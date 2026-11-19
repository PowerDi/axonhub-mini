package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	SystemKeyCatalogSettings = "catalog_settings"

	DefaultCatalogUpstreamURL = "https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/dev/dist/all.json"

	catalogProxyEnvironment = "environment"
	catalogProxyDirect      = "direct"
	catalogProxyPreset      = "preset"
)

// CatalogSettings controls the upstream URL and proxy for model catalog downloads.
type CatalogSettings struct {
	UpstreamURL    string `json:"upstream_url"`
	ProxyMode      string `json:"proxy_mode"`
	ProxyPresetURL string `json:"proxy_preset_url,omitempty"`
}

func normalizeCatalogSettings(settings *CatalogSettings) {
	settings.UpstreamURL = strings.TrimSpace(settings.UpstreamURL)
	settings.ProxyMode = strings.TrimSpace(settings.ProxyMode)
	if settings.ProxyMode == "" {
		settings.ProxyMode = catalogProxyEnvironment
	}
	if settings.ProxyMode != catalogProxyPreset {
		settings.ProxyPresetURL = ""
	}
}

func validateCatalogSettings(settings CatalogSettings) error {
	if settings.UpstreamURL == "" {
		return nil
	}

	parsed, err := url.Parse(settings.UpstreamURL)
	if err != nil {
		return fmt.Errorf("invalid catalog upstream url: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("catalog upstream url must be http or https")
	}

	if parsed.Host == "" {
		return fmt.Errorf("catalog upstream url is missing a host")
	}

	return nil
}

func (settings CatalogSettings) EffectiveUpstreamURL() string {
	if strings.TrimSpace(settings.UpstreamURL) == "" {
		return DefaultCatalogUpstreamURL
	}

	return settings.UpstreamURL
}

func defaultCatalogSettings() CatalogSettings {
	return CatalogSettings{ProxyMode: catalogProxyEnvironment}
}

func (s *SystemService) CatalogSettings(ctx context.Context) (*CatalogSettings, error) {
	value, err := s.getSystemValue(ctx, SystemKeyCatalogSettings)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultCatalogSettings()), nil
		}

		return nil, fmt.Errorf("failed to get catalog settings: %w", err)
	}

	settings := defaultCatalogSettings()
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal catalog settings: %w", err)
	}

	normalizeCatalogSettings(&settings)

	return &settings, nil
}

func (s *SystemService) SetCatalogSettings(ctx context.Context, settings CatalogSettings) error {
	normalizeCatalogSettings(&settings)
	if err := validateCatalogSettings(settings); err != nil {
		return err
	}
	if _, err := s.catalogProxyConfig(ctx, settings); err != nil {
		return err
	}

	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal catalog settings: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyCatalogSettings, string(jsonBytes))
}

func (s *SystemService) catalogProxyConfig(ctx context.Context, settings CatalogSettings) (*httpclient.ProxyConfig, error) {
	switch settings.ProxyMode {
	case catalogProxyEnvironment:
		return nil, nil
	case catalogProxyDirect:
		return &httpclient.ProxyConfig{Type: httpclient.ProxyTypeDisabled}, nil
	case catalogProxyPreset:
		if strings.TrimSpace(settings.ProxyPresetURL) == "" {
			return nil, fmt.Errorf("catalog proxy preset is required")
		}
		presets, err := s.ProxyPresets(ctx)
		if err != nil {
			return nil, fmt.Errorf("read catalog proxy presets: %w", err)
		}
		preset, found := lo.Find(presets, func(preset ProxyPreset) bool {
			return preset.URL == settings.ProxyPresetURL
		})
		if !found {
			return nil, fmt.Errorf("selected catalog proxy preset does not exist")
		}
		parsed, err := url.Parse(preset.URL)
		if err != nil || parsed.Hostname() == "" {
			return nil, fmt.Errorf("selected catalog proxy preset has an invalid URL")
		}
		switch parsed.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return nil, fmt.Errorf("selected catalog proxy preset uses an unsupported scheme")
		}
		return &httpclient.ProxyConfig{
			Type:     httpclient.ProxyTypeURL,
			URL:      preset.URL,
			Username: preset.Username,
			Password: preset.Password,
		}, nil
	default:
		return nil, fmt.Errorf("invalid catalog proxy mode")
	}
}
