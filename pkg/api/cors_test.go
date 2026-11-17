package api

import "testing"

func TestManagementOrigins(t *testing.T) {
	for _, tt := range []struct {
		name    string
		origins []string
		origin  string
		want    bool
	}{
		{"production configured", []string{"https://console.company.test"}, "https://console.company.test", true},
		{"production wrong port", []string{"https://console.company.test"}, "https://console.company.test:8443", false},
		{"production wrong scheme", []string{"https://console.company.test"}, "http://console.company.test", false},
		{"production disallows development", []string{"https://console.company.test"}, "http://localhost:5173", false},
		{"development localhost", nil, "http://localhost:5173", true},
		{"development loopback ipv6", nil, "http://[::1]:5173", true},
		{"development private ipv4", nil, "http://172.16.0.1:5173", true},
		{"public 172 address", nil, "https://172.32.0.1", false},
		{"dns prefix bypass", nil, "https://10.attacker.test", false},
		{"dns private prefix bypass", nil, "https://192.168.attacker.test", false},
		{"public origin needs configuration", nil, "https://console.company.test", false},
		{"userinfo", nil, "http://attacker@localhost", false},
		{"path", nil, "http://localhost/path", false},
		{"null origin", []string{"null"}, "null", false},
		{"wildcard is not an origin", []string{"*"}, "https://attacker.test", false},
		{"scheme", nil, "ftp://localhost", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := managementOriginAllowed(tt.origins)(tt.origin); got != tt.want {
				t.Fatalf("origin %q: allowed=%v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}
