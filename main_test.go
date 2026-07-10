package main

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestValidateTargetURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		target    string
		wantError string
	}{
		{name: "missing URL", target: "", wantError: "missing URL"},
		{name: "invalid URL", target: "://bad", wantError: "invalid URL"},
		{name: "rejects http", target: "http://bugsnag.com/app.js", wantError: "https required"},
		{name: "rejects user info", target: "https://user@bugsnag.com/app.js", wantError: "user info not allowed"},
		{name: "rejects non allowlisted host", target: "https://example.com/app.js", wantError: "host not allowed"},
		{name: "rejects non standard port", target: "https://bugsnag.com:8443/app.js", wantError: "port not allowed"},
		{name: "allows allowlisted host", target: "https://subdomain.bugsnag.com/app.js"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			u, err := validateTargetURL(test.target)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("validateTargetURL returned unexpected error: %v", err)
				}
				if u == nil {
					t.Fatal("validateTargetURL returned nil URL")
				}
				return
			}

			if err == nil {
				t.Fatalf("validateTargetURL returned nil error, want %q", test.wantError)
			}

			if !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("validateTargetURL error = %q, want substring %q", err.Error(), test.wantError)
			}
		})
	}
}

func TestRestrictedDialContextRejectsBlockedResolution(t *testing.T) {
	originalLookup := lookupIPAddr
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
	}
	defer func() {
		lookupIPAddr = originalLookup
	}()

	_, err := restrictedDialContext(context.Background(), "tcp", "bugsnag.com:443")
	if err == nil {
		t.Fatal("restrictedDialContext returned nil error for blocked resolution")
	}
	if !strings.Contains(err.Error(), "resolved IP not allowed") {
		t.Fatalf("restrictedDialContext error = %q, want blocked resolution error", err.Error())
	}
}

func TestIsBlockedIP(t *testing.T) {
	t.Parallel()

	if !isBlockedIP(net.ParseIP("10.0.0.1")) {
		t.Fatal("expected private IP to be blocked")
	}
	if !isBlockedIP(net.ParseIP("169.254.169.254")) {
		t.Fatal("expected metadata IP to be blocked")
	}
	if isBlockedIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("expected public IP to be allowed")
	}
}
