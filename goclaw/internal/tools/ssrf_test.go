package tools

import (
	"net/url"
	"strings"
	"testing"
)

func TestValidateRedirectURLBlocksRFC1918(t *testing.T) {
	for _, raw := range []string{
		"http://192.168.1.1/path",
		"http://10.0.0.1/",
		"http://172.16.0.1/",
	} {
		u, perr := url.Parse(raw)
		if perr != nil {
			t.Fatal(perr)
		}
		verr := validateRedirectURL(u)
		if verr == nil {
			t.Fatalf("validateRedirectURL(%q): expected error", raw)
		}
		if !strings.Contains(verr.Error(), "non-public") {
			t.Fatalf("validateRedirectURL(%q): want blocked address hint, got %v", raw, verr)
		}
	}
}

func TestValidateRedirectURLBlocksLoopbackAndMetadata(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:8080/",
		"http://[::1]/",
		"http://169.254.169.254/latest/meta-data/",
	} {
		u, perr := url.Parse(raw)
		if perr != nil {
			t.Fatal(perr)
		}
		if validateRedirectURL(u) == nil {
			t.Fatalf("validateRedirectURL(%q): expected error", raw)
		}
	}
}

func TestValidateRedirectURLAllowsHTTPSWithPublicHostname(t *testing.T) {
	// Host resolves in real DNS; we only check that literal public hosts pass.
	u, err := url.Parse("https://example.com/path")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRedirectURL(u); err != nil {
		t.Fatalf("validateRedirectURL(example.com): %v", err)
	}
}

func TestValidateRedirectURLRejectsNonHTTPScheme(t *testing.T) {
	u, perr := url.Parse("file:///etc/passwd")
	if perr != nil {
		t.Fatal(perr)
	}
	verr := validateRedirectURL(u)
	if verr == nil || !strings.Contains(verr.Error(), "scheme") {
		t.Fatalf("expected scheme error, got %v", verr)
	}
}
