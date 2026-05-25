package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserFromServiceAdmin_MapsActivityTimestamps(t *testing.T) {
	t.Parallel()

	lastLoginAt := time.Date(2026, time.April, 20, 10, 0, 0, 0, time.UTC)
	lastActiveAt := lastLoginAt.Add(15 * time.Minute)
	lastUsedAt := lastLoginAt.Add(45 * time.Minute)

	out := UserFromServiceAdmin(&service.User{
		ID:           42,
		Email:        "admin@example.com",
		Username:     "admin",
		Role:         service.RoleAdmin,
		Status:       service.StatusActive,
		LastActiveAt: &lastActiveAt,
		LastUsedAt:   &lastUsedAt,
	})

	require.NotNil(t, out)
	require.NotNil(t, out.LastActiveAt)
	require.NotNil(t, out.LastUsedAt)
	require.WithinDuration(t, lastActiveAt, *out.LastActiveAt, time.Second)
	require.WithinDuration(t, lastUsedAt, *out.LastUsedAt, time.Second)
}

func TestUserFromServiceOperator_RedactsAdminOnlyFields(t *testing.T) {
	t.Parallel()

	out := UserFromServiceOperator(&service.User{
		ID:         43,
		Email:      "operator-view@example.com",
		Role:       service.RoleUser,
		Status:     service.StatusActive,
		Notes:      "internal note",
		APIKeys:    []service.APIKey{{ID: 1, Key: "sk-secret"}},
		GroupRates: map[int64]float64{7: 1.25},
	})

	require.NotNil(t, out)
	require.Equal(t, "", out.Notes)
	require.Nil(t, out.GroupRates)
	require.Nil(t, out.APIKeys)
	require.Equal(t, "operator-view@example.com", out.Email)
}
