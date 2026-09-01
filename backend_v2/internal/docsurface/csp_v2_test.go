package docsurface

import (
	"strings"
	"testing"
)

func TestV2CSPOnlyAddsRuntimeCompilation(t *testing.T) {
	for _, policy := range []string{CSP, CSPWithVendor("https://aladin.example"), CSPWithVendorScheme()} {
		if CSPForBridgeVersion(policy, "bridge/1") != policy {
			t.Fatal("v1 policy changed")
		}
		if CSPForBridgeVersion(policy, "untrusted") != policy {
			t.Fatal("unknown version relaxed policy")
		}
		actual := CSPForBridgeVersion(policy, "bridge/2")
		if !strings.Contains(actual, "'unsafe-eval'") {
			t.Fatal("v2 compilation is blocked")
		}
		if strings.Replace(actual, " 'unsafe-eval'", "", 1) != policy {
			t.Fatal("unrelated policy changed")
		}
	}
}
