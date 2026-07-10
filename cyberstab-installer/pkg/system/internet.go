package system

import (
	"context"
	"net"
	"net/http"
	"time"
)

var internetProbeURLs = []string{
	"http://www.msftconnecttest.com/connecttest.txt",
	"http://clients3.google.com/generate_204",
	"https://www.cloudflare.com/cdn-cgi/trace",
}

// HasInternetAccess reports whether the machine can reach the public internet.
func HasInternetAccess() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   1500 * time.Millisecond,
				KeepAlive: 0,
			}).DialContext,
			TLSHandshakeTimeout: 1500 * time.Millisecond,
			DisableKeepAlives:   true,
		},
	}

	for _, probeURL := range internetProbeURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return true
		}
	}
	return false
}
