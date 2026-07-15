// Package arr contains shared Sonarr/Radarr client foundations.
package arr

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/httputil"
)

type Service string

const (
	Sonarr Service = "sonarr"
	Radarr Service = "radarr"
)

type Instance struct {
	Service           Service
	Name, URL, APIKey string
}
type Client struct{ HTTP *httputil.Client }

// Get decodes one authenticated v3 API resource.
func (c Client) Get(ctx context.Context, instance Instance, resource string, query url.Values, result any) error {
	if c.HTTP == nil {
		return fmt.Errorf("ARR HTTP client is nil")
	}
	base := strings.TrimSuffix(instance.URL, "/")
	endpoint := base + "/api/v3/" + strings.TrimPrefix(resource, "/")
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	if err := c.HTTP.DoJSON(ctx, "GET", endpoint, map[string]string{"X-Api-Key": instance.APIKey}, nil, result); err != nil {
		return fmt.Errorf("%s %s: %w", instance.Service, instance.Name, err)
	}
	return nil
}

type SystemStatus struct {
	AppName string `json:"appName"`
	Version string `json:"version"`
}

func (c Client) Status(ctx context.Context, instance Instance) (SystemStatus, error) {
	var status SystemStatus
	err := c.Get(ctx, instance, "system/status", nil, &status)
	return status, err
}
