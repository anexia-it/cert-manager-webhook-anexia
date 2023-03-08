# Solver testdata directory

To run the integration tests, you need to insert a base64 encoded Anexia Engine token
with read/write access to the Anexia CloudDNS API into `anexia-clouddns-secret.yml`.
The token needs to have access to the CloudDNS zone you specify in `TEST_ZONE_NAME`.

```
ANEXIA_API_TOKEN_BASE64="$(echo -n "<my-anexia-api-token>" | base64)"; \
  sed -i "s/changemeplease/${ANEXIA_API_TOKEN_BASE64}/g" testdata/anexia/anexia-clouddns-secret.yml
```