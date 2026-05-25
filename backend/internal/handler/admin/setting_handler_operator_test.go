package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOperatorSettingsResponseDataRedactsSensitiveKeys(t *testing.T) {
	got := operatorSettingsResponseData(map[string]any{
		"site_name":                "Sub2API",
		"default_balance":          float64(10),
		"smtp_host":                "smtp.example.com",
		"turnstile_site_key":       "site-key",
		"payment_enabled":          true,
		"risk_control_enabled":     true,
		"ops_monitoring_enabled":   true,
		"openai_codex_user_agent":  "codex",
		"admin_api_key_configured": true,
	})

	require.Equal(t, "Sub2API", got["site_name"])
	require.Equal(t, float64(10), got["default_balance"])
	require.NotContains(t, got, "smtp_host")
	require.NotContains(t, got, "turnstile_site_key")
	require.NotContains(t, got, "payment_enabled")
	require.NotContains(t, got, "risk_control_enabled")
	require.NotContains(t, got, "ops_monitoring_enabled")
	require.NotContains(t, got, "openai_codex_user_agent")
	require.NotContains(t, got, "admin_api_key_configured")
}

func TestOperatorSettingsUpdateRequestKeepsAdminOnlyFields(t *testing.T) {
	previous := &service.SystemSettings{
		SiteName:                         "old site",
		SMTPHost:                         "smtp.example.com",
		SMTPPort:                         465,
		SMTPUsername:                     "smtp-user",
		SMTPPassword:                     "smtp-secret",
		TurnstileEnabled:                 true,
		TurnstileSiteKey:                 "turnstile-site",
		TurnstileSecretKey:               "turnstile-secret",
		BackendModeEnabled:               true,
		RiskControlEnabled:               true,
		OpsMonitoringEnabled:             true,
		EnableFingerprintUnification:     true,
		PaymentVisibleMethodAlipaySource: "official_alipay",
		DefaultBalance:                   5,
		DefaultSubscriptions: []service.DefaultSubscriptionSetting{
			{GroupID: 1, ValidityDays: 30},
		},
	}
	authDefaults := &service.AuthSourceDefaultSettings{
		Email: service.ProviderDefaultGrantSettings{
			Balance:       1,
			Concurrency:   5,
			Subscriptions: []service.DefaultSubscriptionSetting{{GroupID: 1, ValidityDays: 30}},
		},
	}

	input := UpdateSettingsRequest{
		SiteName:                      "new site",
		DefaultBalance:                20,
		DefaultSubscriptions:          []dto.DefaultSubscriptionSetting{{GroupID: 2, ValidityDays: 60}},
		SMTPHost:                      "attacker.example.com",
		TurnstileEnabled:              false,
		TurnstileSiteKey:              "changed-site",
		TurnstileSecretKey:            "changed-secret",
		BackendModeEnabled:            false,
		RiskControlEnabled:            settingsBoolPtr(false),
		OpsMonitoringEnabled:          settingsBoolPtr(false),
		PaymentEnabled:                settingsBoolPtr(true),
		PaymentEnabledTypes:           []string{"stripe"},
		AuthSourceDefaultEmailBalance: settingsFloat64Ptr(9),
	}

	got := operatorSettingsUpdateRequest(input, previous, authDefaults)

	require.Equal(t, "new site", got.SiteName)
	require.Equal(t, float64(20), got.DefaultBalance)
	require.Equal(t, []dto.DefaultSubscriptionSetting{{GroupID: 2, ValidityDays: 60}}, got.DefaultSubscriptions)
	require.Equal(t, float64(9), *got.AuthSourceDefaultEmailBalance)
	require.Equal(t, "smtp.example.com", got.SMTPHost)
	require.Equal(t, 465, got.SMTPPort)
	require.Equal(t, "smtp-secret", got.SMTPPassword)
	require.True(t, got.TurnstileEnabled)
	require.Equal(t, "turnstile-site", got.TurnstileSiteKey)
	require.Equal(t, "turnstile-secret", got.TurnstileSecretKey)
	require.True(t, got.BackendModeEnabled)
	require.True(t, *got.RiskControlEnabled)
	require.True(t, *got.OpsMonitoringEnabled)
	require.True(t, *got.EnableFingerprintUnification)
	require.Equal(t, "official_alipay", *got.PaymentVisibleMethodAlipaySource)
	require.Nil(t, got.PaymentEnabled)
	require.Nil(t, got.PaymentEnabledTypes)
}
