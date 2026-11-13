package models

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestPluginSettingsNeverSerializeSecureToken(t *testing.T) {
	const token = "never-serialize-this-token"
	settings, err := LoadPluginSettings(backend.DataSourceInstanceSettings{
		URL: "http://coremetry:8088",
		DecryptedSecureJSONData: map[string]string{
			"apiToken": token,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), token) {
		t.Fatal("serialized datasource settings exposed the API token")
	}
}
