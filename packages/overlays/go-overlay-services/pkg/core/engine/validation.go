package engine

import (
	"net/url"
	"strings"
)

// IsValidHostingURL validates a URL to ensure it does not match disallowed patterns:
// - Contains "http:" protocol (only https is allowed)
// - Contains "localhost" (with or without a port)
// - Internal or non-routable IP addresses (e.g., 192.168.x.x, 10.x.x.x, 172.16.x.x to 172.31.x.x)
// - Non-routable IPs like 127.x.x.x, 0.0.0.0, or IPv6 loopback (::1)
func IsValidHostingURL(hostingURL string) bool {
	if hostingURL == "" {
		return false
	}

	parsedURL, err := url.Parse(hostingURL)
	if err != nil {
		return false
	}

	// Disallow http:
	if parsedURL.Scheme == "http" {
		return false
	}

	// Require a valid scheme
	if parsedURL.Scheme == "" {
		return false
	}

	hostname := parsedURL.Hostname()

	// Disallow localhost (case-insensitive)
	if strings.EqualFold(hostname, "localhost") {
		return false
	}

	// Check for non-routable IPv4 addresses
	if isNonRoutableIPv4(hostname) {
		return false
	}

	// Check for IPv6 loopback
	if hostname == "::1" || hostname == "[::1]" {
		return false
	}

	// Also check Host field for IPv6 without brackets
	if parsedURL.Host == "::1" {
		return false
	}

	return true
}

// isNonRoutableIPv4 checks if the given IP string is a non-routable IPv4 address.
// This includes loopback (127.x.x.x), private ranges (10.x.x.x, 192.168.x.x, 172.16-31.x.x),
// and the all-zeros address (0.0.0.0).
func isNonRoutableIPv4(ip string) bool {
	// Pattern matching for non-routable IPv4 addresses
	patterns := []struct {
		prefix string
		check  func(string) bool
	}{
		{"127.", func(ip string) bool { return len(ip) >= 4 && ip[:4] == "127." }},
		{"10.", func(ip string) bool { return len(ip) >= 3 && ip[:3] == "10." }},
		{"192.168.", func(ip string) bool { return len(ip) >= 8 && ip[:8] == "192.168." }},
		{"0.0.0.0", func(ip string) bool { return ip == "0.0.0.0" }},
	}

	for _, pattern := range patterns {
		if pattern.check(ip) {
			return true
		}
	}

	// Check for 172.16.x.x to 172.31.x.x
	if len(ip) >= 7 && ip[:4] == "172." {
		// Extract second octet
		secondOctet := ""
		for i := 4; i < len(ip) && ip[i] != '.'; i++ {
			secondOctet += string(ip[i])
		}
		if octet := parseOctet(secondOctet); octet >= 16 && octet <= 31 {
			return true
		}
	}

	return false
}

// parseOctet parses a string as an IPv4 octet (0-255).
// Returns -1 if the string is not a valid octet.
func parseOctet(s string) int {
	if s == "" {
		return -1
	}
	val := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return -1
		}
		val = val*10 + int(ch-'0')
		if val > 255 {
			return -1
		}
	}
	return val
}
