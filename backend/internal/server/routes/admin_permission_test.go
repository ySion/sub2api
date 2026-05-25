package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminPermissionRoutesAllowOperatorOnOperationalRoutes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		path       string
		assertCall func(t *testing.T, svc *adminPermissionRouteService)
	}{
		{name: "announcements", path: "/api/v1/admin/announcements"},
		{
			name: "users",
			path: "/api/v1/admin/users",
			assertCall: func(t *testing.T, svc *adminPermissionRouteService) {
				require.True(t, svc.listUsersCalled)
			},
		},
		{
			name: "groups",
			path: "/api/v1/admin/groups",
			assertCall: func(t *testing.T, svc *adminPermissionRouteService) {
				require.True(t, svc.listGroupsCalled)
			},
		},
		{
			name: "redeem_codes",
			path: "/api/v1/admin/redeem-codes",
			assertCall: func(t *testing.T, svc *adminPermissionRouteService) {
				require.True(t, svc.listRedeemCodesCalled)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &adminPermissionRouteService{}
			router := newAdminPermissionRouteRouter(service.RoleOperator, svc)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			if tc.assertCall != nil {
				tc.assertCall(t, svc)
			}
		})
	}
}

func TestAdminPermissionRoutesRejectOperatorOnSensitiveRoutes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "accounts", method: http.MethodGet, path: "/api/v1/admin/accounts"},
		{name: "proxies", method: http.MethodGet, path: "/api/v1/admin/proxies"},
		{name: "settings_admin_api_key", method: http.MethodGet, path: "/api/v1/admin/settings/admin-api-key"},
		{name: "settings_smtp_test", method: http.MethodPost, path: "/api/v1/admin/settings/test-smtp"},
		{name: "settings_overload_cooldown", method: http.MethodGet, path: "/api/v1/admin/settings/overload-cooldown"},
		{name: "settings_web_search", method: http.MethodGet, path: "/api/v1/admin/settings/web-search-emulation"},
		{name: "user_api_keys", method: http.MethodGet, path: "/api/v1/admin/users/1/api-keys"},
		{name: "user_role", method: http.MethodPut, path: "/api/v1/admin/users/1/role"},
		{name: "group_api_keys", method: http.MethodGet, path: "/api/v1/admin/groups/1/api-keys"},
		{name: "redeem_codes_export", method: http.MethodGet, path: "/api/v1/admin/redeem-codes/export"},
		{name: "redeem_codes_generate", method: http.MethodPost, path: "/api/v1/admin/redeem-codes/generate"},
		{name: "redeem_codes_create_and_redeem", method: http.MethodPost, path: "/api/v1/admin/redeem-codes/create-and-redeem"},
		{name: "redeem_codes_delete", method: http.MethodDelete, path: "/api/v1/admin/redeem-codes/1"},
		{name: "redeem_codes_batch_delete", method: http.MethodPost, path: "/api/v1/admin/redeem-codes/batch-delete"},
		{name: "redeem_codes_batch_update", method: http.MethodPost, path: "/api/v1/admin/redeem-codes/batch-update"},
		{name: "usage_search_api_keys", method: http.MethodGet, path: "/api/v1/admin/usage/search-api-keys"},
		{name: "affiliate_user_config", method: http.MethodGet, path: "/api/v1/admin/affiliates/users"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &adminPermissionRouteService{}
			router := newAdminPermissionRouteRouter(service.RoleOperator, svc)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusForbidden, w.Code)
			require.False(t, svc.listAccountsCalled)
			require.False(t, svc.listProxiesCalled)
			require.False(t, svc.listRedeemCodesCalled)
		})
	}
}

func TestAdminPermissionRoutesAllowOperatorOperationalMutations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "promo_codes_create", method: http.MethodPost, path: "/api/v1/admin/promo-codes", body: ""},
		{name: "promo_codes_update", method: http.MethodPut, path: "/api/v1/admin/promo-codes/1", body: ""},
		{name: "promo_codes_delete", method: http.MethodDelete, path: "/api/v1/admin/promo-codes/not-number"},
		{name: "subscriptions_assign", method: http.MethodPost, path: "/api/v1/admin/subscriptions/assign", body: ""},
		{name: "subscriptions_bulk_assign", method: http.MethodPost, path: "/api/v1/admin/subscriptions/bulk-assign", body: ""},
		{name: "subscriptions_extend", method: http.MethodPost, path: "/api/v1/admin/subscriptions/not-number/extend", body: "{}"},
		{name: "subscriptions_reset_quota", method: http.MethodPost, path: "/api/v1/admin/subscriptions/not-number/reset-quota", body: "{}"},
		{name: "subscriptions_revoke", method: http.MethodDelete, path: "/api/v1/admin/subscriptions/not-number"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &adminPermissionRouteService{}
			router := newAdminPermissionRouteRouter(service.RoleOperator, svc)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(w, req)

			require.NotEqual(t, http.StatusForbidden, w.Code)
		})
	}
}

func TestPaymentOrderMutationRoutesRejectOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "cancel", method: http.MethodPost, path: "/api/v1/admin/payment/orders/1/cancel"},
		{name: "retry", method: http.MethodPost, path: "/api/v1/admin/payment/orders/1/retry"},
		{name: "refund", method: http.MethodPost, path: "/api/v1/admin/payment/orders/1/refund"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			router := gin.New()
			adminGroup := router.Group("/api/v1/admin/payment")
			adminGroup.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUserRole), service.RoleOperator)
				c.Next()
			})
			adminOnly := adminGroup.Group("")
			adminOnly.Use(middleware.AdminOnly())

			ordersAdminOnly := adminOnly.Group("/orders")
			ordersAdminOnly.POST("/:id/cancel", func(c *gin.Context) { called = true })
			ordersAdminOnly.POST("/:id/retry", func(c *gin.Context) { called = true })
			ordersAdminOnly.POST("/:id/refund", func(c *gin.Context) { called = true })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusForbidden, w.Code)
			require.False(t, called)
		})
	}
}

func TestPaymentPlanMutationRoutesAllowOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "create", method: http.MethodPost, path: "/api/v1/admin/payment/plans"},
		{name: "update", method: http.MethodPut, path: "/api/v1/admin/payment/plans/1"},
		{name: "delete", method: http.MethodDelete, path: "/api/v1/admin/payment/plans/1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			router := gin.New()
			adminGroup := router.Group("/api/v1/admin/payment")
			adminGroup.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUserRole), service.RoleOperator)
				c.Next()
			})

			plans := adminGroup.Group("/plans")
			plans.POST("", func(c *gin.Context) { called = true })
			plans.PUT("/:id", func(c *gin.Context) { called = true })
			plans.DELETE("/:id", func(c *gin.Context) { called = true })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			require.True(t, called)
		})
	}
}

func TestAdminPermissionRoutesAllowAdminOnSensitiveRoutes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		path       string
		assertCall func(t *testing.T, svc *adminPermissionRouteService)
	}{
		{
			name: "accounts",
			path: "/api/v1/admin/accounts",
			assertCall: func(t *testing.T, svc *adminPermissionRouteService) {
				require.True(t, svc.listAccountsCalled)
			},
		},
		{
			name: "proxies",
			path: "/api/v1/admin/proxies",
			assertCall: func(t *testing.T, svc *adminPermissionRouteService) {
				require.True(t, svc.listProxiesCalled)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &adminPermissionRouteService{}
			router := newAdminPermissionRouteRouter(service.RoleAdmin, svc)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			tc.assertCall(t, svc)
		})
	}
}

func newAdminPermissionRouteRouter(role string, svc *adminPermissionRouteService) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	admin := router.Group("/api/v1/admin")
	admin.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserRole), role)
		c.Next()
	})
	adminOnly := admin.Group("")
	adminOnly.Use(middleware.AdminOnly())

	h := &handler.Handlers{
		Admin: &handler.AdminHandlers{
			User:          adminhandler.NewUserHandler(svc, nil),
			Group:         adminhandler.NewGroupHandler(svc, nil, nil),
			Account:       adminhandler.NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil),
			OAuth:         adminhandler.NewOAuthHandler(nil),
			Announcement:  adminhandler.NewAnnouncementHandler(service.NewAnnouncementService(&adminPermissionAnnouncementRepo{}, nil, nil, nil)),
			Proxy:         adminhandler.NewProxyHandler(svc),
			Redeem:        adminhandler.NewRedeemHandler(svc, nil),
			Promo:         adminhandler.NewPromoHandler(nil),
			Setting:       adminhandler.NewSettingHandler(nil, nil, nil, nil, nil, nil, nil),
			Usage:         adminhandler.NewUsageHandler(nil, nil, svc, nil),
			UserAttribute: adminhandler.NewUserAttributeHandler(nil),
			Affiliate:     adminhandler.NewAffiliateHandler(nil, svc),
		},
	}

	registerAnnouncementRoutes(admin, h)
	registerUserManagementRoutes(admin, adminOnly, h)
	registerGroupRoutes(admin, adminOnly, h)
	registerAccountRoutes(adminOnly, h)
	registerRedeemCodeRoutes(admin, adminOnly, h)
	registerPromoCodeRoutes(admin, adminOnly, h)
	registerProxyRoutes(adminOnly, h)
	registerSettingsRoutes(admin, adminOnly, h)
	registerSubscriptionRoutes(admin, adminOnly, h)
	registerUsageRoutes(admin, adminOnly, h)
	registerAffiliateRoutes(admin, adminOnly, h)

	return router
}

type adminPermissionRouteService struct {
	service.AdminService

	listUsersCalled       bool
	listGroupsCalled      bool
	listAccountsCalled    bool
	listProxiesCalled     bool
	listRedeemCodesCalled bool
}

func (s *adminPermissionRouteService) ListUsers(ctx context.Context, page, pageSize int, filters service.UserListFilters, sortBy, sortOrder string) ([]service.User, int64, error) {
	s.listUsersCalled = true
	return []service.User{}, 0, nil
}

func (s *adminPermissionRouteService) ListGroups(ctx context.Context, page, pageSize int, platform, status, search string, isExclusive *bool, sortBy, sortOrder string) ([]service.Group, int64, error) {
	s.listGroupsCalled = true
	return []service.Group{}, 0, nil
}

func (s *adminPermissionRouteService) ListRedeemCodes(ctx context.Context, page, pageSize int, codeType, status, search string, sortBy, sortOrder string) ([]service.RedeemCode, int64, error) {
	s.listRedeemCodesCalled = true
	return []service.RedeemCode{}, 0, nil
}

func (s *adminPermissionRouteService) ListAccounts(ctx context.Context, page, pageSize int, platform, accountType, status, search string, groupID int64, privacyMode string, sortBy, sortOrder string) ([]service.Account, int64, error) {
	s.listAccountsCalled = true
	return []service.Account{}, 0, nil
}

func (s *adminPermissionRouteService) ListProxiesWithAccountCount(ctx context.Context, page, pageSize int, protocol, status, search string, sortBy, sortOrder string) ([]service.ProxyWithAccountCount, int64, error) {
	s.listProxiesCalled = true
	return []service.ProxyWithAccountCount{}, 0, nil
}

type adminPermissionAnnouncementRepo struct{}

func (r *adminPermissionAnnouncementRepo) Create(ctx context.Context, a *service.Announcement) error {
	panic("unexpected Create call")
}

func (r *adminPermissionAnnouncementRepo) GetByID(ctx context.Context, id int64) (*service.Announcement, error) {
	panic("unexpected GetByID call")
}

func (r *adminPermissionAnnouncementRepo) Update(ctx context.Context, a *service.Announcement) error {
	panic("unexpected Update call")
}

func (r *adminPermissionAnnouncementRepo) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (r *adminPermissionAnnouncementRepo) List(ctx context.Context, params pagination.PaginationParams, filters service.AnnouncementListFilters) ([]service.Announcement, *pagination.PaginationResult, error) {
	return []service.Announcement{}, &pagination.PaginationResult{Total: 0, Page: params.Page, PageSize: params.PageSize, Pages: 0}, nil
}

func (r *adminPermissionAnnouncementRepo) ListActive(ctx context.Context, now time.Time) ([]service.Announcement, error) {
	panic("unexpected ListActive call")
}
