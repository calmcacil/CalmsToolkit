package arr

import (
	"context"
	"github.com/calmcacil/CalmsToolkit/internal/httputil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/system/status" || r.Header.Get("X-Api-Key") != "secret" {
			t.Errorf("request=%s key=%q", r.URL.Path, r.Header.Get("X-Api-Key"))
		}
		w.Write([]byte(`{"appName":"Sonarr","version":"4"}`))
	}))
	defer server.Close()
	status, err := (Client{HTTP: httputil.NewClient(time.Second)}).Status(context.Background(), Instance{Service: Sonarr, Name: "HD", URL: server.URL, APIKey: "secret"})
	if err != nil || status.AppName != "Sonarr" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}
