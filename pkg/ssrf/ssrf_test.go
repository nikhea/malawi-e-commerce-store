package ssrf_test

import (
	"context"
	"testing"

	"github.com/nikhea/malawi-e-commerce-store/pkg/ssrf"
)

func TestValidateURL(t *testing.T) {
	blocked := []string{
		"", "not a url", "ftp://files.example.com/x.jpg",
		"http://user:pass@cdn.example.com/x.jpg",
		"http://127.0.0.1/x.jpg",
		"http://localhost/x.jpg",
		"http://10.0.0.5/x.jpg",
		"http://172.16.9.9/x.jpg",
		"http://192.168.1.1/admin",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/x.jpg",
		"http://0.0.0.0/x.jpg",
		"file:///etc/passwd",
	}
	for _, raw := range blocked {
		t.Run("blocked "+raw, func(t *testing.T) {
			if err := ssrf.ValidateURL(context.Background(), raw); err == nil {
				t.Fatalf("expected block for %q", raw)
			}
		})
	}

	allowed := []string{
		// Literal IPs only: hostname allows would need DNS, breaking
		// hermetic test runs. The resolve path is covered by the
		// localhost-blocked case above.
		"https://8.8.8.8/x.jpg",
		"http://1.1.1.1:8080/a/b.jpg?q=1",
	}
	for _, raw := range allowed {
		t.Run("allowed "+raw, func(t *testing.T) {
			if err := ssrf.ValidateURL(context.Background(), raw); err != nil {
				t.Fatalf("expected allow for %q: %v", raw, err)
			}
		})
	}
}
