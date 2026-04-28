package engine_test

import (
	"testing"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
)

func TestIsValidHostingURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		// Valid URLs
		{"valid public HTTPS URL", "https://example.com", true},
		{"valid public HTTPS URL with port", "https://example.com:8080", true},
		{"valid public HTTPS URL with path", "https://example.com/path", true},
		{"valid IP HTTPS URL", "https://1.2.3.4", true},

		// Invalid URLs - HTTP protocol
		{"HTTP protocol", "http://example.com", false},
		{"HTTP with port", "http://example.com:8080", false},

		// Invalid URLs - localhost
		{"localhost", "https://localhost", false},
		{"localhost with port", "https://localhost:8080", false},
		{"localhost uppercase", "https://LOCALHOST", false},
		{"localhost mixed case", "https://LocalHost:3000", false},

		// Invalid URLs - loopback addresses
		{"loopback 127.0.0.1", "https://127.0.0.1", false},
		{"loopback 127.0.0.1 with port", "https://127.0.0.1:8080", false},
		{"loopback 127.1.2.3", "https://127.1.2.3", false},
		{"IPv6 loopback", "https://[::1]", false},
		{"IPv6 loopback without brackets", "https://::1", false},

		// Invalid URLs - private IP ranges
		{"private IP 10.x", "https://10.0.0.1", false},
		{"private IP 10.x with port", "https://10.255.255.255:8080", false},
		{"private IP 192.168.x", "https://192.168.1.1", false},
		{"private IP 192.168.x with port", "https://192.168.0.1:3000", false},
		{"private IP 172.16.x", "https://172.16.0.1", false},
		{"private IP 172.31.x", "https://172.31.255.255", false},
		{"private IP 172.15.x (valid)", "https://172.15.0.1", true},
		{"private IP 172.32.x (valid)", "https://172.32.0.1", true},

		// Invalid URLs - non-routable
		{"non-routable 0.0.0.0", "https://0.0.0.0", false},

		// Invalid URLs - malformed
		{"empty string", "", false},
		{"invalid URL", "not-a-url", false},
		{"missing protocol", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.IsValidHostingURL(tt.url)
			if got != tt.expected {
				t.Errorf("IsValidHostingURL(%q) = %v, want %v", tt.url, got, tt.expected)
			}
		})
	}
}
