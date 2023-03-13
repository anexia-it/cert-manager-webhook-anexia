package main

import (
	"os"
	"testing"
	"time"

	"github.com/cert-manager/cert-manager/test/acme/dns"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
	fqdn = os.Getenv("TEST_FQDN")
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.
	//

	fixture := dns.NewFixture(&anexiaDNSProviderSolver{},
		dns.SetResolvedZone(zone),
		dns.SetResolvedFQDN(fqdn),
		dns.SetDNSServer("acns01.xaas.systems:53"),
		dns.SetPollInterval(10*time.Second),
		dns.SetPropagationLimit(10*time.Minute),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/anexia"),
	)
	//need to uncomment and  RunConformance delete runBasic and runExtended once https://github.com/cert-manager/cert-manager/pull/4835 is merged
	//fixture.RunConformance(t)
	fixture.RunBasic(t)
	fixture.RunExtended(t)

}
