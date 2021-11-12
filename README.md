[![codecov](https://codecov.io/gh/anexia-it/cert-manager-webhook-anexia/branch/main/graph/badge.svg?token=G573SD12QG)](https://codecov.io/gh/anexia-it/cert-manager-webhook-anexia)

# cert-manager ACME webhook for Anexia CloudDNS

This repository provides a webhook to use cert-manager with Anexia CloudDNS.

## Requirements
-   [go](https://golang.org/) >= 1.13.0
-   [helm](https://helm.sh/) >= v3.0.0
-   [cert-manager](https://cert-manager.io/) >= 1.6.0

## Installation

### cert-manager

Follow the [instructions](https://cert-manager.io/docs/installation/) in the cert-manager documentation to install it within your cluster.

### Webhook

#### Using public helm chart
```bash
helm repo add cert-manager-webhook-anexia https://anexia-it.github.io/cert-manager-webhook-anexia
helm install --namespace cert-manager cert-manager-webhook-anexia cert-manager-webhook-anexia/cert-manager-webhook-anexia
```

#### From local checkout

```bash
helm install --namespace cert-manager cert-manager-webhook-anexia deploy/cert-manager-webhook-anexia
```
**Note**: Only installing the webhook's K8s resources to the same namespace as the cert-manager is tested.

To uninstall the webhook run
```bash
helm uninstall --namespace cert-manager cert-manager-webhook-anexia
```
## Usage

### Issuer

Create a `ClusterIssuer` or `Issuer` resource as follows:
```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: anexia-issuer
spec:
  acme:
    email: mail@example.com # REPLACE THIS WITH YOUR EMAIL!!!
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: anexia-issuer-account-key
    solvers:
    - dns01:
        webhook:
          groupName: acme.anexia.com
          solverName: anexia
          config:
            secretRef: anexia-clouddns-secret # The Secret resource storing the Anexia Engine token to interact with CloudDNS
            secretRefNamespace: cert-manager # The namespace where the secret lives
            secretKey: anexia-token # The key used for the token entry in the data section of the secret
```

### Credentials
In order to access the CloudDNS API, the webhook needs an Anexia Engine API token.

If you choose a different name for the secret than `anexia-clouddns-secret`,
make sure that you modify the value of `secretRef` in the `[Cluster]Issuer` config section.
If you want to keep the secret in a different namespace than `cert-manager`,
make sure that the ServiceAccount has access to it.

The secret for the example above may look like this:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: anexia-clouddns-secret
  namespace: cert-manager
data:
  anexia-token: <base64 encoded Anexia Engine token>
```

### Create a certificate

Finally you can create certificates, for example:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: anexia-cert-manager-test
spec:
  # Secret names are always required.
  secretName: anexia-cert-manager-test-tls

  duration: 2160h # 90d
  renewBefore: 360h # 15d
  subject:
    organizations:
      - YourOrg
  isCA: false
  privateKey:
    algorithm: RSA
    encoding: PKCS1
    size: 2048
  usages:
    - server auth
    - client auth
  # At least one of a DNS Name, URI, or IP address is required.
  dnsNames:
    - example.com
  # Issuer references are always required.
  issuerRef:
    name: anexia-issuer
    # We can reference ClusterIssuers by changing the kind here.
    # The default value is Issuer (i.e. a locally namespaced Issuer)
    kind: ClusterIssuer
```

## Development

### Running the test suite

This solver implementation **must** pass the DNS01 provider conformance testing suite.

See [testdata/anexia/README.md] for details on how to set up the test configuration and secret.
You'll need an Anexia Engine token with CloudDNS access and a zone since these are integration tests
running against the real CloudDNS API.

You can run the test suite with:

```bash
TEST_ZONE_NAME=example.com. make test
```

By also setting `TEST_FQDN=specific.$TEST_ZONE_NAME`
you can specify the exact name for which a record will be presented,
which is useful when running concurrent integration tests.

## Credits

We would like to thank all contributors to this project, some of which are not included in the git history:
* [Simon](https://github.com/kaisers1)
* [Christoph Glantschnig](https://github.com/glantscc)
* Sebastian Kerin
* [Tobias Paepke](https://github.com/paepke)
* [Roland Urbano](https://github.com/X4mp)
