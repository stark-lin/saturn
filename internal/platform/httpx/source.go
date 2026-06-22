// This file derives auditable request source information from HTTP requests.
package httpx

import (
	"context"
	"net"
	"net/http"
	"strings"
)

const loopbackSourceIP = "127.0.0.1"

type RequestSource struct {
	IP        string
	UserAgent string
}

const requestSourceKey contextKey = "request_source"

func CaptureRequestSource(trustedProxyCIDRs []string, next http.Handler) http.Handler {
	trustedProxies := make([]*net.IPNet, 0, len(trustedProxyCIDRs))
	for _, cidr := range trustedProxyCIDRs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err == nil {
			trustedProxies = append(trustedProxies, network)
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteIP := requestRemoteIP(r.RemoteAddr)
		sourceIP := remoteIP
		if matchesNetwork(remoteIP, trustedProxies) {
			if forwardedIP := firstForwardedIP(r.Header.Get("X-Forwarded-For")); forwardedIP != "" {
				sourceIP = forwardedIP
			}
		}
		source := RequestSource{
			IP:        normalizeSourceIP(sourceIP),
			UserAgent: strings.TrimSpace(r.UserAgent()),
		}
		ctx := context.WithValue(r.Context(), requestSourceKey, source)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestSourceFromContext(ctx context.Context) RequestSource {
	source, ok := ctx.Value(requestSourceKey).(RequestSource)
	if !ok || source.IP == "" {
		return RequestSource{IP: loopbackSourceIP}
	}
	return source
}

func requestRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err == nil {
		return host
	}
	if net.ParseIP(strings.TrimSpace(remoteAddr)) != nil {
		return strings.TrimSpace(remoteAddr)
	}
	return loopbackSourceIP
}

func firstForwardedIP(header string) string {
	for _, value := range strings.Split(header, ",") {
		candidate := strings.TrimSpace(value)
		if net.ParseIP(candidate) != nil {
			return candidate
		}
	}
	return ""
}

func matchesNetwork(value string, networks []*net.IPNet) bool {
	ip := net.ParseIP(value)
	if ip == nil {
		return false
	}
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func normalizeSourceIP(value string) string {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil || ip.IsLoopback() {
		return loopbackSourceIP
	}
	return ip.String()
}
