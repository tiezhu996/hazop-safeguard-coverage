package hzgv007

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"hazop-safeguard-coverage/backend/internal/constants"
	"hazop-safeguard-coverage/backend/internal/dto"
	"hazop-safeguard-coverage/backend/internal/handler"
	"hazop-safeguard-coverage/backend/internal/middleware"
	"hazop-safeguard-coverage/backend/internal/router"
	"hazop-safeguard-coverage/backend/internal/util"
)

func TestAuditorReadPermissionRestored(t *testing.T) {
	engine := safeguardEngine(t, "auditor")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/safeguards", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("auditor read status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestReviewerCanVerifySafeguard(t *testing.T) {
	engine := safeguardEngine(t, "safety_reviewer")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/safeguards/1/verify", strings.NewReader(`{"verified_at":"2026-08-24T00:00:00Z","evidence_note":"proof test ok"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("reviewer verify status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestReviewerCanCreateSafeguard(t *testing.T) {
	engine := safeguardEngine(t, "safety_reviewer")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/safeguards", strings.NewReader(`{"name":"Layer","safeguard_type":"alarm","target_scenario_id":1,"independence_key":"KEY-1","effectiveness":0.8,"test_interval_days":180,"evidence_note":"proof evidence"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("reviewer create status = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestProcessEngineerCannotInvalidateSafeguard(t *testing.T) {
	engine := safeguardEngine(t, "process_engineer")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/safeguards/1/invalidate", strings.NewReader(`{"reason":"field change required"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("process_engineer invalidate status = %d, want 403; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAuditorCanUseExplicitRoleRoute(t *testing.T) {
	engine := safeguardEngine(t, "auditor")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auditor-only", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("auditor explicit role status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

func safeguardEngine(t *testing.T, role string) *gin.Engine {
	t.Helper()
	safeguardHandler := handler.NewSafeguardHandler(&stubSafeguardService{})
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("actor", util.Actor{UserID: 1, Username: "user", Role: role, RequestID: "req-rbac"})
		c.Next()
	})
	api := engine.Group("/api/v1")
	router.RegisterSafeguardRoutes(api, safeguardHandler)
	api.GET("/auditor-only", middleware.RequireRoles(constants.RoleAuditor), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return engine
}

type stubSafeguardService struct{}

func (s *stubSafeguardService) Create(ctx context.Context, request dto.CreateSafeguardRequest, actor util.Actor) (dto.SafeguardResponse, error) {
	return dto.SafeguardResponse{ID: 1, Name: request.Name}, nil
}

func (s *stubSafeguardService) Get(ctx context.Context, id uint) (dto.SafeguardResponse, error) {
	return dto.SafeguardResponse{ID: id}, nil
}

func (s *stubSafeguardService) List(ctx context.Context, query dto.SafeguardQuery) (dto.SafeguardListResponse, error) {
	return dto.SafeguardListResponse{Page: query.Page, Size: query.PageSize}, nil
}

func (s *stubSafeguardService) Update(ctx context.Context, id uint, request dto.UpdateSafeguardRequest, actor util.Actor) (dto.SafeguardResponse, error) {
	return dto.SafeguardResponse{ID: id}, nil
}

func (s *stubSafeguardService) Verify(ctx context.Context, id uint, request dto.VerifySafeguardRequest, actor util.Actor) (dto.SafeguardResponse, error) {
	return dto.SafeguardResponse{ID: id}, nil
}

func (s *stubSafeguardService) Invalidate(ctx context.Context, id uint, request dto.SafeguardActionRequest, actor util.Actor) (dto.SafeguardResponse, error) {
	return dto.SafeguardResponse{ID: id}, nil
}

func (s *stubSafeguardService) Restore(ctx context.Context, id uint, request dto.SafeguardActionRequest, actor util.Actor) (dto.SafeguardResponse, error) {
	return dto.SafeguardResponse{ID: id}, nil
}
