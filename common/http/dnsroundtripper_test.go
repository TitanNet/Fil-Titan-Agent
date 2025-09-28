package http

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestDnsRoundTripper(t *testing.T) {
	// os.Setenv("TITAN_DOH_SERVER", "https://dns.podoe.cn")
	// url := "https://www-test1-api.titannet.io/api/nodeinfo?nodeid=xxxxx"
	url := "https://pcdn.titannet.io"
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: DefaultDNSRountTripper,
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
}
