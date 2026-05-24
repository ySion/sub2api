package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserHandlerUpdateRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &userRoleAdminService{stubAdminService: newStubAdminService()}
	router := newUserRoleRouter(1, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/2/role", bytes.NewReader([]byte(`{"role":"operator"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(2), svc.updatedUserID)
	require.Equal(t, service.RoleOperator, svc.updatedRole)
}

func TestUserHandlerUpdateRoleRejectsSelfChange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &userRoleAdminService{stubAdminService: newStubAdminService()}
	router := newUserRoleRouter(2, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/2/role", bytes.NewReader([]byte(`{"role":"operator"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Zero(t, svc.updatedUserID)
}

func TestUserHandlerUpdateRoleRejectsInvalidRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &userRoleAdminService{stubAdminService: newStubAdminService()}
	router := newUserRoleRouter(1, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/2/role", bytes.NewReader([]byte(`{"role":"owner"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Zero(t, svc.updatedUserID)
}

func newUserRoleRouter(actorID int64, svc service.AdminService) *gin.Engine {
	router := gin.New()
	h := NewUserHandler(svc, nil)
	router.PUT("/api/v1/admin/users/:id/role", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: actorID})
		c.Next()
	}, h.UpdateRole)
	return router
}

type userRoleAdminService struct {
	*stubAdminService
	updatedUserID int64
	updatedRole   string
}

func (s *userRoleAdminService) UpdateUserRole(ctx context.Context, id int64, role string) (*service.User, error) {
	s.updatedUserID = id
	s.updatedRole = role
	return &service.User{ID: id, Email: "updated@example.com", Role: role, Status: service.StatusActive}, nil
}
