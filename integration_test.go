//go:build integration

package main

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/cert-manager/cert-manager/test/acme"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/client"
	"k8s.io/klog/v2"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
	fqdn = os.Getenv("TEST_FQDN")
)

func init() {
	// Register klog flags; the cert-manager conformance fixture can't do this
	// itself in a test binary, so its verbose logging never kicks in. Without
	// this, verbosity stays at 0 and the useful debug/DNS logs are hidden.
	klog.InitFlags(nil)
	_ = flag.Set("logtostderr", "true")
	_ = flag.Set("v", "6")
}

func TestCertManagerSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.
	//

	if zone == "" {
		t.Fatal("cannot run integration tests without env TEST_ZONE_NAME")
	}
	if fqdn == "" {
		fqdn = "localtest." + zone
	}

	fixture := dns.NewFixture(&anexiaDNSProviderSolver{
		getEngineClient: func(token string) (api.API, error) {
			return api.NewAPI(
				api.WithClientOptions(
					client.TokenFromString(token),
				),
			)
		},
	},
		dns.SetResolvedZone(zone),
		dns.SetResolvedFQDN(fqdn),
		dns.SetDNSServer("acns01.xaas.systems:53"),
		dns.SetPollInterval(10*time.Second),
		dns.SetPropagationLimit(10*time.Minute),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/anexia"),
	)
	fixture.RunConformance(t)
}
