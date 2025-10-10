package http

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/miekg/dns"
)

var DefaultDNSRountTripper = NewDNSRoundTripper()

// DnsCache stores DNS resolution results in cache
type DnsCache struct {
	IP     string
	TTL    time.Duration
	Expire time.Time
}

// DNSRoundTripper custom RoundTripper for DNS resolution
type DNSRoundTripper struct {
	Transport http.RoundTripper
	cache     map[string]*DnsCache
	// dohServer string
}

// NewDNSRoundTripper creates a new DNSRoundTripper instance
func NewDNSRoundTripper() *DNSRoundTripper {
	rt := &DNSRoundTripper{
		cache: make(map[string]*DnsCache),
		// dohServer: os.Getenv("TITAN_DOH_SERVER"),
	}

	rt.Transport = &http.Transport{
		DialContext: rt.dailContext,
	}

	return rt
}

// dailContext handles DNS resolution and creates a tcp/udp connection
func (d *DNSRoundTripper) dailContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// arr := strings.Split(addr, ":")
	// hostname, port := arr[0], arr[1]
	hostname, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.Printf("DNSRoundTripper.dailContext Cannot split host and port: %v", err)
		return net.Dial(network, addr)
	}

	// Check cache first
	if cacheEntry, exists := d.cache[hostname]; exists && time.Now().Before(cacheEntry.Expire) {
		return net.Dial(network, cacheEntry.IP+":"+port)
	}

	// Query DNS
	ip, ttl, err := d.queryDNS(hostname)
	if err != nil {
		// ip, err = d.queryLocalDNS(hostname)
		// if err != nil {
		// 	return nil, fmt.Errorf("DNS lookup failed: %v", err)
		// }
		// ttl = time.Minute // Use short TTL for local DNS
		return net.Dial(network, addr) // using default resolver instead of local-cached resolver
	}

	// Update cache
	d.cache[hostname] = &DnsCache{
		IP:     ip,
		TTL:    ttl,
		Expire: time.Now().Add(ttl),
	}

	return net.Dial(network, ip+":"+port)
}

func (d *DNSRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return d.Transport.RoundTrip(req)
}

// queryDNS queries DoH server for DNS resolution
func (d *DNSRoundTripper) queryDNS(domain string) (string, time.Duration, error) {
	dohserver := os.Getenv("TITAN_DOH_SERVER")
	if dohserver == "" {
		return "", 0, fmt.Errorf("DoH server not configured, falling back to local DNS")
	}

	// Build DNS query
	dnsMsg := d.createDNSQuery(domain)

	// Base64 encode the query
	encodedQuery := base64.RawURLEncoding.EncodeToString(dnsMsg)

	// Construct DoH request URL
	url := fmt.Sprintf("%s/dns-query?dns=%s", dohserver, encodedQuery)

	// Send GET request to DoH server
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", 0, fmt.Errorf("failed to query DoH server: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	responseBytes := make([]byte, resp.ContentLength)
	_, err = resp.Body.Read(responseBytes)
	if err != nil && err != io.EOF {
		return "", 0, fmt.Errorf("failed to read DoH response body: %v", err)
	}

	// Parse DNS response
	ip, ttl, err := d.parseDNSResponse(responseBytes)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse DoH response: %v", err)
	}

	return ip, ttl, nil
}

// createDNSQuery creates a DNS query message for A record
func (d *DNSRoundTripper) createDNSQuery(domain string) []byte {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	msg.RecursionDesired = true
	msg.Authoritative = true

	// Pack DNS message
	query, err := msg.Pack()
	if err != nil {
		log.Fatalf("Failed to pack DNS query: %v", err)
	}

	return query
}

// parseDNSResponse parses DNS response and extracts IP and TTL
func (d *DNSRoundTripper) parseDNSResponse(response []byte) (string, time.Duration, error) {
	dnsMsg := new(dns.Msg)
	err := dnsMsg.Unpack(response)
	if err != nil {
		return "", 0, fmt.Errorf("failed to unpack DNS response: %v", err)
	}

	// Extract A record IP address
	for _, answer := range dnsMsg.Answer {
		if aRecord, ok := answer.(*dns.A); ok {
			ttl := time.Duration(aRecord.Hdr.Ttl) * time.Second
			return aRecord.A.String(), ttl, nil
		}
	}

	return "", 0, fmt.Errorf("no A record found in DNS response")
}

// queryLocalDNS uses local DNS to resolve domain's IP address
func (d *DNSRoundTripper) queryLocalDNS(domain string) (string, error) {
	ips, err := net.LookupHost(domain)
	if err != nil {
		return "", err
	}
	return ips[0], nil
}
