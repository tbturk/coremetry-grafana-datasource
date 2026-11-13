package models

import (
	"encoding/json"
	"errors"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type PluginSettings struct {
	BaseURL  string `json:"-"`
	APIToken string `json:"-"`
}

func LoadPluginSettings(source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	if len(source.JSONData) > 0 && !json.Valid(source.JSONData) {
		return nil, errors.New("invalid datasource settings")
	}

	return &PluginSettings{
		BaseURL:  source.URL,
		APIToken: source.DecryptedSecureJSONData["apiToken"],
	}, nil
}
