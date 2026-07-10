package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/bugsnag/bugsnag-go"
)

func main() {

	bugsnag.Configure(bugsnag.Configuration{
		APIKey: "953611040d6cfb7f10a9aec0ae81b845",
	})

	http.Handle("/bower_components/", http.StripPrefix("/bower_components/", http.FileServer(http.Dir("bower_components"))))

	http.HandleFunc("/get/", proxy)

	http.Handle("/", http.FileServer(http.Dir("public")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}

	log.Println("Listening on :" + port)

	err := http.ListenAndServe(":"+port, bugsnag.Handler(nil))
	if err != nil {
		panic(err)
	}
}

var allowedDomains = []string{
	"smartbear.com",
	"bugsnag.com",
}

var lookupIPAddr = net.DefaultResolver.LookupIPAddr

var outboundHTTPClient = &http.Client{Transport: newRestrictedTransport()}

type approvedTarget struct {
	host     string
	path     string
	rawQuery string
}

func newRestrictedTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = restrictedDialContext
	return transport
}

func isAllowedHost(host string) bool {
	host = strings.ToLower(host)
	for _, domain := range allowedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}

	return ip.Equal(net.ParseIP("169.254.169.254"))
}

func (target approvedTarget) url() *url.URL {
	path := target.path
	if path == "" {
		path = "/"
	}

	return &url.URL{
		Scheme:   "https",
		Host:     target.host,
		Path:     path,
		RawQuery: target.rawQuery,
	}
}

func validateTargetURL(target string) (*approvedTarget, error) {
	if target == "" {
		return nil, fmt.Errorf("missing URL")
	}

	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid URL")
	}

	if u.Scheme != "https" {
		return nil, fmt.Errorf("https required")
	}

	if u.User != nil {
		return nil, fmt.Errorf("user info not allowed")
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("host required")
	}

	if !isAllowedHost(host) {
		return nil, fmt.Errorf("host not allowed")
	}

	if port := u.Port(); port != "" && port != "443" {
		return nil, fmt.Errorf("port not allowed")
	}

	if ip := net.ParseIP(host); ip != nil && isBlockedIP(ip) {
		return nil, fmt.Errorf("IP address not allowed")
	}

	return &approvedTarget{
		host:     host,
		path:     u.EscapedPath(),
		rawQuery: u.RawQuery,
	}, nil
}

func restrictedDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	host = strings.ToLower(host)
	if !isAllowedHost(host) {
		return nil, fmt.Errorf("host not allowed")
	}

	ipAddrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ipAddrs) == 0 {
		return nil, fmt.Errorf("host did not resolve")
	}

	for _, ipAddr := range ipAddrs {
		if isBlockedIP(ipAddr.IP) {
			return nil, fmt.Errorf("resolved IP not allowed")
		}
	}

	var dialer net.Dialer
	return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
}

func proxy(w http.ResponseWriter, r *http.Request) {

	//req, err := http.NewRequest("GET", r.URL.Query().Get("url"), nil)
	target := r.URL.Query().Get("url")
	approved, err := validateTargetURL(target)
	if err != nil {
		statusCode := http.StatusBadRequest
		if err.Error() == "host not allowed" || err.Error() == "IP address not allowed" || err.Error() == "port not allowed" {
			statusCode = http.StatusForbidden
		}
		http.Error(w, err.Error(), statusCode)
		return
	}

	req := &http.Request{
		Method: http.MethodGet,
		URL:    approved.url(),
		Header: make(http.Header),
	}
	req = req.WithContext(r.Context())

	// Make it easy for upstreams to filter out traffic from sourcemaps.info
	// We should also deploy this with a static outbound IP.
	req.Header.Set("User-Agent", "proxy.sourcemaps.info/v1.0 (conrad@bugsnag.com)")

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	req.Header.Set("X-Forwarded-For", ip)
	req.Header.Set("X-SourceMapsInfo-User", r.RemoteAddr)

	log.Printf("Fetching %s for %s\n", target, r.RemoteAddr)

	resp, err := outboundHTTPClient.Do(req)

	if err != nil {
		w.Header().Set("X-Proxy-Error", err.Error())
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(err.Error()))
		return
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-SourceMap") != "" {
		w.Header().Set("X-SourceMap", resp.Header.Get("X-SourceMap"))
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
