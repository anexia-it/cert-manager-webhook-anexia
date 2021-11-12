package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"

	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
	"github.com/jetstack/cert-manager/pkg/issuer/acme/dns/util"

	anxcloudClient "github.com/anexia-it/go-anxcloud/pkg/client"
	anxcloudZone "github.com/anexia-it/go-anxcloud/pkg/clouddns/zone"
)

var GroupName = os.Getenv("GROUP_NAME")

const Timeout = 30 * time.Second

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(GroupName,
		&anexiaDNSProviderSolver{},
	)
}

// anexiaDNSProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface.
type anexiaDNSProviderSolver struct {
	client *kubernetes.Clientset
}

// anexiaDNSProviderConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
// This information is provided by cert-manager, and may be a reference to
// additional configuration that's needed to solve the challenge for this
// particular certificate or issuer.
// This typically includes references to Secret resources containing DNS
// provider credentials, in cases where a 'multi-tenant' DNS solver is being
// created.
// If you do *not* require per-issuer or per-certificate configuration to be
// provided to your webhook, you can skip decoding altogether in favour of
// using CLI flags or similar to provide configuration.
// You should not include sensitive information here. If credentials need to
// be used by your provider here, you should reference a Kubernetes Secret
// resource and fetch these credentials using a Kubernetes clientset.
type anexiaDNSProviderConfig struct {
	SecretRef          string `json:"secretRef"`
	SecretRefNamespace string `json:"secretRefNamespace"`
	SecretKey          string `json:"secretKey"`
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *anexiaDNSProviderSolver) Name() string {
	return "anexia"
}

// Find a TXT record with the given recordName and recordRData in the CloudDNS zone with the name zoneName
func FindTXTRecord(c anxcloudClient.Client, ctx context.Context, zoneName string, recordName string, recordRData string) (*anxcloudZone.Record, error) {
	zoneAPI := anxcloudZone.NewAPI(c)
	records, err := zoneAPI.ListRecords(ctx, zoneName)

	if err != nil {
		return nil, err
	}

	for _, r := range records {
		// Work around ENGSUP-5257, CloudDNS API is inserting quotes into the TXT record data
		if strings.HasPrefix(r.RData, "\"") && strings.HasSuffix(r.RData, "\"") {
			r.RData = strings.TrimPrefix(r.RData, "\"")
			r.RData = strings.TrimSuffix(r.RData, "\"")
		}

		if r.Name == recordName && r.RData == recordRData && r.Type == "TXT" {
			return &r, nil
		}
	}
	return nil, nil
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *anexiaDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	token, err := getToken(cfg, c, ch)
	if err != nil {
		klog.Error("Unable to aquire anxcloud token:", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	client, err := anxcloudClient.New(anxcloudClient.TokenFromString(token))
	if err != nil {
		klog.Error("Unable to set up anxcloud client:", err)
		return err
	}

	zoneName := util.UnFqdn(ch.ResolvedZone)
	recordName := util.UnFqdn(strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone))

	// Look if the needed record already exists
	r, err := FindTXTRecord(client, ctx, zoneName, recordName, ch.Key)
	if err != nil {
		klog.Error("Unable to search in existing records:", err)
		return err
	}

	// If a record with the given specs already exists and was found, we don't need to do anything
	// The CloudDNS API will error out if we try to recreate an existing record, so we have to return here
	if r != nil {
		return nil
	}

	recordRequest := anxcloudZone.RecordRequest{
		Name:  recordName,
		Type:  "TXT",
		RData: ch.Key,
		TTL:   120,
	}

	_, err = anxcloudZone.NewAPI(client).NewRecord(ctx, zoneName, recordRequest)
	if err != nil {
		klog.Error(err)
		klog.Error("Unable to create record, RecordRequest was:", recordRequest)
		return err
	}

	klog.Info("Created a record for ", ch.ResolvedFQDN)
	return nil
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *anexiaDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	token, err := getToken(cfg, c, ch)
	if err != nil {
		klog.Error("Unable to aquire anxcloud token:", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	client, err := anxcloudClient.New(anxcloudClient.TokenFromString(token))
	if err != nil {
		klog.Error("Unable to set up anxcloud client:", err)
		return err
	}

	zoneName := util.UnFqdn(ch.ResolvedZone)
	recordName := util.UnFqdn(strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone))

	r, err := FindTXTRecord(client, ctx, zoneName, recordName, ch.Key)
	if err != nil {
		klog.Error("Unable to search in existing records:", err)
		return err
	}

	if r != nil {
		zoneAPI := anxcloudZone.NewAPI(client)
		err := zoneAPI.DeleteRecord(ctx, util.UnFqdn(ch.ResolvedZone), r.Identifier)
		if err != nil {
			klog.Error("Unable to delete record:", err)
			return err
		}
		klog.Info("Deleted a record for ", ch.ResolvedFQDN)
		return nil
	}

	return fmt.Errorf("could not find and delete record for %s", ch.ResolvedFQDN)
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *anexiaDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.client = cl

	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (anexiaDNSProviderConfig, error) {
	cfg := anexiaDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}

func getToken(cfg anexiaDNSProviderConfig, c *anexiaDNSProviderSolver, ch *v1alpha1.ChallengeRequest) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	secret, err := c.client.CoreV1().Secrets(cfg.SecretRefNamespace).Get(ctx, cfg.SecretRef, metav1.GetOptions{})

	if err != nil {
		klog.Errorf("Unable to get secret %s in namespace %s: %v", cfg.SecretRef, cfg.SecretRefNamespace, err)
		return "", err
	}

	token_data, ok := secret.Data[cfg.SecretKey]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret data", cfg.SecretKey)
	}
	return string(token_data), nil
}
