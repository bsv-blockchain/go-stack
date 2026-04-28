package engine

import (
	"testing"
)

// FuzzIsValidHostingURL fuzzes the IsValidHostingURL function to discover edge cases
// in URL parsing and validation logic. This tests for panics, unexpected errors,
// and validates the function's robustness against malformed input.
func FuzzIsValidHostingURL(f *testing.F) {
	// Seed corpus with valid URLs
	f.Add("https://example.com")
	f.Add("https://example.com:8080")
	f.Add("https://example.com/path")
	f.Add("https://1.2.3.4")
	f.Add("https://172.15.0.1")
	f.Add("https://172.32.0.1")

	// Seed corpus with invalid URLs - http protocol
	f.Add("http://example.com")
	f.Add("http://example.com:8080")

	// Seed corpus with invalid URLs - localhost
	f.Add("https://localhost")
	f.Add("https://localhost:8080")
	f.Add("https://LOCALHOST")
	f.Add("https://LocalHost:3000")

	// Seed corpus with invalid URLs - loopback addresses
	f.Add("https://127.0.0.1")
	f.Add("https://127.0.0.1:8080")
	f.Add("https://127.1.2.3")
	f.Add("https://[::1]")
	f.Add("https://::1")

	// Seed corpus with invalid URLs - private IP ranges
	f.Add("https://10.0.0.1")
	f.Add("https://10.255.255.255:8080")
	f.Add("https://192.168.1.1")
	f.Add("https://192.168.0.1:3000")
	f.Add("https://172.16.0.1")
	f.Add("https://172.31.255.255")

	// Seed corpus with invalid URLs - non-routable
	f.Add("https://0.0.0.0")

	// Seed corpus with malformed inputs
	f.Add("")
	f.Add("not-a-url")
	f.Add("example.com")
	f.Add("://example.com")
	f.Add("https://")
	f.Add("https:///")
	f.Add("https://[")
	f.Add("https://]")
	f.Add("https://[:]")

	// Edge cases for IP parsing
	f.Add("https://172.1.0.1")
	f.Add("https://172.16.0.1")
	f.Add("https://172.31.0.1")
	f.Add("https://172.32.0.1")
	f.Add("https://256.256.256.256")
	f.Add("https://192.168.")
	f.Add("https://10.")

	f.Fuzz(func(t *testing.T, url string) {
		// The function should never panic, regardless of input
		result := IsValidHostingURL(url)

		// The result should always be a boolean (this validates no panic occurred)
		_ = result

		// Additional invariant: empty string should always be invalid
		if url == "" && result {
			t.Errorf("IsValidHostingURL(%q) returned true, expected false for empty string", url)
		}

		// Invariant: URLs with "http:" scheme should always be invalid
		if len(url) >= 5 && url[:5] == "http:" && result {
			t.Errorf("IsValidHostingURL(%q) returned true, expected false for http:// URL", url)
		}
	})
}

// FuzzParseOctet fuzzes the parseOctet function to test IPv4 octet parsing robustness.
// This validates proper handling of edge cases in numeric string parsing.
func FuzzParseOctet(f *testing.F) {
	// Valid octets
	f.Add("0")
	f.Add("1")
	f.Add("127")
	f.Add("255")
	f.Add("16")
	f.Add("31")

	// Invalid octets - out of range
	f.Add("256")
	f.Add("999")
	f.Add("1000")

	// Invalid octets - non-numeric
	f.Add("")
	f.Add("a")
	f.Add("1a")
	f.Add("a1")
	f.Add("-1")
	f.Add("12.3")

	// Edge cases
	f.Add("00")
	f.Add("01")
	f.Add("001")
	f.Add("0000")

	f.Fuzz(func(t *testing.T, octetStr string) {
		// The function should never panic
		result := parseOctet(octetStr)

		// Invariants
		// Empty string should return -1
		if octetStr == "" && result != -1 {
			t.Errorf("parseOctet(%q) = %d, expected -1 for empty string", octetStr, result)
		}

		// Result should be in valid octet range or -1 for invalid input
		if result < -1 || result > 255 {
			t.Errorf("parseOctet(%q) = %d, expected value in [-1, 255]", octetStr, result)
		}

		// If result is valid (not -1), all characters should be digits
		if result != -1 {
			for _, ch := range octetStr {
				if ch < '0' || ch > '9' {
					t.Errorf("parseOctet(%q) = %d, expected -1 for non-digit input", octetStr, result)
					break
				}
			}
		}
	})
}

// FuzzIsNonRoutableIPv4 fuzzes the isNonRoutableIPv4 function to test IP validation logic.
func FuzzIsNonRoutableIPv4(f *testing.F) {
	// Routable IPs
	f.Add("1.2.3.4")
	f.Add("8.8.8.8")
	f.Add("172.15.0.1")
	f.Add("172.32.0.1")
	f.Add("11.0.0.1")
	f.Add("193.168.1.1")

	// Non-routable IPs
	f.Add("127.0.0.1")
	f.Add("127.1.2.3")
	f.Add("10.0.0.1")
	f.Add("10.255.255.255")
	f.Add("192.168.1.1")
	f.Add("192.168.0.1")
	f.Add("172.16.0.1")
	f.Add("172.31.255.255")
	f.Add("0.0.0.0")

	// Edge cases
	f.Add("")
	f.Add("172.")
	f.Add("172.16")
	f.Add("172.16.")
	f.Add("172.a.0.1")
	f.Add("256.256.256.256")
	f.Add("localhost")
	f.Add("::1")

	f.Fuzz(func(t *testing.T, ip string) {
		// The function should never panic
		result := isNonRoutableIPv4(ip)

		// The result should always be a boolean
		_ = result

		// Invariant: known non-routable prefixes
		if len(ip) >= 4 && ip[:4] == "127." && !result {
			t.Errorf("isNonRoutableIPv4(%q) = false, expected true for 127.x loopback", ip)
		}
		if len(ip) >= 3 && ip[:3] == "10." && !result {
			t.Errorf("isNonRoutableIPv4(%q) = false, expected true for 10.x private range", ip)
		}
		if len(ip) >= 8 && ip[:8] == "192.168." && !result {
			t.Errorf("isNonRoutableIPv4(%q) = false, expected true for 192.168.x private range", ip)
		}
		if ip == "0.0.0.0" && !result {
			t.Errorf("isNonRoutableIPv4(%q) = false, expected true for all-zeros address", ip)
		}
	})
}
