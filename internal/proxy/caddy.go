package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type CaddyAdminClient struct {
	BaseURL     string
	HTTPClient  *http.Client
	MaxAttempts int
	RetryDelay  time.Duration
}

func (client CaddyAdminClient) ApplyConfig(ctx context.Context, config CaddyConfig) error {
	baseURL := strings.TrimRight(client.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:2019"
	}
	body, err := json.Marshal(toCaddyAdminConfig(config))
	if err != nil {
		return fmt.Errorf("encode caddy config: %w", err)
	}

	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	maxAttempts := client.MaxAttempts
	retryDelay := client.RetryDelay
	if maxAttempts <= 0 {
		maxAttempts = 20
		if retryDelay == 0 {
			retryDelay = 500 * time.Millisecond
		}
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/load", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create caddy request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				if err := sleepForRetry(ctx, retryDelay); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("apply caddy config: %w", lastErr)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return fmt.Errorf("caddy admin returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
		return nil
	}
	return nil
}

func sleepForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func toCaddyAdminConfig(config CaddyConfig) map[string]any {
	routes := make([]any, 0, len(config.HTTP.Routes))
	for _, route := range config.HTTP.Routes {
		routes = append(routes, map[string]any{
			"match": []any{
				map[string]any{"host": []string{route.Hostname}},
			},
			"handle": []any{
				map[string]any{
					"handler": "reverse_proxy",
					"upstreams": []any{
						map[string]any{"dial": route.UpstreamDial},
					},
				},
			},
			"terminal": true,
		})
	}

	return map[string]any{
		"admin": map[string]any{
			"listen": "0.0.0.0:2019",
		},
		"apps": map[string]any{
			"http": map[string]any{
				"servers": map[string]any{
					"porter": map[string]any{
						"listen":                  []string{":80", ":443"},
						"routes":                  routes,
						"tls_connection_policies": []any{map[string]any{}},
					},
				},
			},
			"tls": map[string]any{
				"automation": map[string]any{
					"on_demand": map[string]any{
						"permission": map[string]any{
							"module":   "http",
							"endpoint": config.HTTP.AskURL,
						},
					},
					"policies": []any{
						map[string]any{"on_demand": true},
					},
				},
			},
		},
	}
}
