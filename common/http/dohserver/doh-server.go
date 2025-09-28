package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"gopkg.in/yaml.v2"
)

const (
	certFile = "./podoe.cn.crt"
	keyFile  = "./podoe.cn.key"
)

type Config struct {
	Port                   int                 `yaml:"port"`
	HttpPort               int                 `yaml:"httpPort"`
	TrustedDomainResolvers map[string][]string `yaml:"trusted_domain_resolvers"`
	CertFile               string              `yaml:"cert_file"`
	KeyFile                string              `yaml:"key_file"`
	UpstreamDNS            string              `yaml:"upstream_dns"`
}

type DoHHandler struct {
	upstreamDNS      string
	trustedResolvers map[string][]string
}

func (h *DoHHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodPost:
		h.handlePost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *DoHHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	dnsParam := r.URL.Query().Get("dns")
	if dnsParam == "" {
		http.Error(w, "Missing 'dns' query parameter", http.StatusBadRequest)
		return
	}

	msg, err := base64.RawURLEncoding.DecodeString(dnsParam)
	if err != nil {
		http.Error(w, "Invalid base64 encoding", http.StatusBadRequest)
		return
	}

	h.processDNSMessage(w, msg)
}

func (h *DoHHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/dns-message" {
		http.Error(w, "Unsupported content type", http.StatusUnsupportedMediaType)
		return
	}

	msg, err := readRequestBody(r)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	h.processDNSMessage(w, msg)
}

func (h *DoHHandler) processDNSMessage(w http.ResponseWriter, msg []byte) {
	dnsMsg := new(dns.Msg)
	if err := dnsMsg.Unpack(msg); err != nil {
		http.Error(w, "Invalid DNS message", http.StatusBadRequest)
		return
	}

	q := dnsMsg.Question[0]
	name := strings.TrimSuffix(q.Name, ".")

	if _, ok := h.trustedResolvers[name]; ok {
		h.processTrustDomain(w, q)
		return
	}

	// for domain, list := range h.trustedResolvers {
	// 	if qname == domain || strings.HasSuffix(qname, "."+domain) {
	// 		resolvers = list
	// 		break
	// 	}
	// }

	// Forward to upstream DNS server
	client := new(dns.Client)
	response, _, err := client.Exchange(dnsMsg, h.upstreamDNS)
	if err != nil {
		http.Error(w, "DNS query failed", http.StatusBadGateway)
		return
	}

	responseBytes, err := response.Pack()
	if err != nil {
		http.Error(w, "Failed to pack DNS response", http.StatusInternalServerError)
		return
	}

	log.Printf("query: %s, result: %s\n", q.Name, response.String())

	w.Header().Set("Content-Type", "application/dns-message")
	w.Write(responseBytes)
}

func (h *DoHHandler) processTrustDomain(w http.ResponseWriter, q dns.Question) {
	ips := h.trustedResolvers[strings.TrimSuffix(q.Name, ".")]

	dnsMsg := new(dns.Msg)
	resp := new(dns.Msg)
	resp.SetReply(dnsMsg)
	resp.Authoritative = true

	for _, ipstr := range ips {
		ip := net.ParseIP(ipstr)
		if ip == nil {
			log.Printf("invalid IP in TrustedDomainResolvers for %s: %s", q.Name, ipstr)
			continue
		}

		var rr dns.RR
		if ip.To4() != nil && q.Qtype == dns.TypeA {
			rr = &dns.A{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    1800,
				},
				A: ip,
			}
		} else if ip.To16() != nil && q.Qtype == dns.TypeAAAA {
			rr = &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    1800,
				},
				AAAA: ip,
			}
		} else {
			// 如果查询类型是 ANY，也同时把 A/AAAA 都返回
			if q.Qtype == dns.TypeANY {
				if ip4 := ip.To4(); ip4 != nil {
					rr = &dns.A{
						Hdr: dns.RR_Header{
							Name:   q.Name,
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    1800,
						},
						A: ip4,
					}
				} else {
					rr = &dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   q.Name,
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    1800,
						},
						AAAA: ip,
					}
				}
			}
		}

		if rr != nil {
			resp.Answer = append(resp.Answer, rr)
		}
	}

	out, err := resp.Pack()
	if err != nil {
		http.Error(w, "Failed to pack DNS response", http.StatusInternalServerError)
		return
	}

	log.Printf("query: %s, result: %s\n", q.Name, resp.String())

	w.Header().Set("Content-Type", "application/dns-message")
	w.Write(out)
}

func readRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	buf := make([]byte, 4096) // DNS messages are typically small
	n, err := r.Body.Read(buf)
	if err != nil && n == 0 {
		return nil, err
	}
	return buf[:n], nil
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func startTLSServer(addr string, handler http.Handler) *http.Server {
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Starting TLS server on %s\n", addr)
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Fatalf("TLS server error: %v\n", err)
		}
	}()

	return srv
}

func startHTTPServer(addr string, handler http.Handler) *http.Server {
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Starting HTTP server on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v\n", err)
		}
	}()

	return srv
}

var configFile = flag.String("f", "config.yaml", "the config file")

func main() {

	flag.Parse()
	var c Config

	file, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("Error opening config file: %v\n", err)
	}

	if err := yaml.Unmarshal(file, &c); err != nil {
		log.Fatalf("Error parsing config file: %v\n", err)
	}

	upstreamDNS := os.Getenv("UPSTREAM_DNS")
	if upstreamDNS == "" {
		upstreamDNS = c.UpstreamDNS
	}
	if upstreamDNS == "" {
		upstreamDNS = "223.5.5.5:53"
	}

	// Create DoH handler
	dohHandler := &DoHHandler{upstreamDNS: upstreamDNS, trustedResolvers: c.TrustedDomainResolvers}

	mux := http.NewServeMux()
	mux.Handle("/dns-query", dohHandler)
	mux.HandleFunc("/health", healthCheckHandler)

	tlsServer := startTLSServer(fmt.Sprintf(":%d", c.Port), mux)

	var httpServer *http.Server
	if c.HttpPort != 0 {
		httpServer = startHTTPServer(fmt.Sprintf(":%d", c.HttpPort), mux)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := tlsServer.Shutdown(ctx); err != nil {
		log.Fatalf("TLS server shutdown error: %v\n", err)
	}

	if httpServer != nil {
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Fatalf("HTTP server shutdown error: %v\n", err)
		}
	}

	log.Println("Servers stopped")
}
