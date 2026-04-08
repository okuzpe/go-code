package tools

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// errPrivateNetwork is returned when a URL targets a blocked address.
var errPrivateNetwork = fmt.Errorf("url targets a non-public address")

// validateHTTPURLForFetch checks scheme/host and resolves the host to ensure
// no blocked IPs (RFC1918, loopback, metadata, IPv6 loopback/link-local).
// ValidateWebhookURL applies the same host/IP rules as web_fetch for outbound HTTP hooks.
func ValidateWebhookURL(raw string) error {
	_, err := validateHTTPURLForFetch(raw)
	return err
}

func validateHTTPURLForFetch(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("only http and https URLs are allowed")
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}
	if err := checkHostResolvedIPs(host); err != nil {
		return nil, err
	}
	return u, nil
}

// validateRedirectURL re-validates a URL before following a redirect.
func validateRedirectURL(ubl *url.URL) error {
	if ubl.Scheme != "http" && ubl.Scheme != "https" {
		return fmt.Errorf("redirect to non-http(s) scheme")
	}
	host := ubl.Hostname()
	if host == "" {
		return fmt.Errorf("redirect missing host")
	}
	return checkHostResolvedIPs(host)
}

func checkHostResolvedIPs(host string) error {
	if ip := net.ParseIP(host); ip != nil {
		return checkIP(ip)
	}
	if strings.EqualFold(host, "localhost") {
		return errPrivateNetwork
	}
	if host == "169.254.169.254" {
		return errPrivateNetwork
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no addresses for host")
	}
	for _, ip := range ips {
		if err := checkIP(ip); err != nil {
			return err
		}
	}
	return nil
}

func checkIP(ip net.IP) error {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return errPrivateNetwork
	}
	if ip.To4() != nil {
		b := ip.To4()
		if b[0] == 169 && b[1] == 254 {
			return errPrivateNetwork
		}
	}
	return nil
}
