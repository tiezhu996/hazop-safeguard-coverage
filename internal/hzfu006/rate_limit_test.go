package hzfu006

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"hazop-safeguard-coverage/backend/internal/dto"
	"hazop-safeguard-coverage/backend/internal/handler"
	"hazop-safeguard-coverage/backend/internal/middleware"
	"hazop-safeguard-coverage/backend/internal/router"
	"hazop-safeguard-coverage/backend/internal/util"
)

func TestRateLimiterConcurrentRequestsNoRace(t *testing.T) {
	limiter := middleware.NewRateLimiter(10000)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/limit", limiter.Middleware("race-scope"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	start := make(chan struct{})
	var wg sync.WaitGroup
	for worker := 0; worker < 20; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < 20; i++ {
				request := httptest.NewRequest(http.MethodGet, "/limit", nil)
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(recorder, request)
				if recorder.Code != http.StatusNoContent {
					t.Errorf("limit request status = %d, want 204", recorder.Code)
				}
			}
		}()
	}
	close(start)
	wg.Wait()
}

func TestReplayUsesSeparateRateScope(t *testing.T) {
	limiter := middleware.NewRateLimiter(2)
	engine := coverageEngine(t, "process_engineer", limiter)
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/coverage-evaluations", strings.NewReader(`{"scenario_id":1}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "run-key-0001")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("run request %d status = %d, want 201", i, recorder.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coverage-evaluations/1/replay", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("replay status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestListDoesNotConsumeRunScope(t *testing.T) {
	limiter := middleware.NewRateLimiter(1)
	engine := coverageEngine(t, "process_engineer", limiter)
	runRequest := httptest.NewRequest(http.MethodPost, "/api/v1/coverage-evaluations", strings.NewReader(`{"scenario_id":1}`))
	runRequest.Header.Set("Content-Type", "application/json")
	runRequest.Header.Set("Idempotency-Key", "run-list-0001")
	runRecorder := httptest.NewRecorder()
	engine.ServeHTTP(runRecorder, runRequest)
	if runRecorder.Code != http.StatusCreated {
		t.Fatalf("run status = %d, want 201", runRecorder.Code)
	}
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/coverage-evaluations", nil)
	listRecorder := httptest.NewRecorder()
	engine.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list after run status = %d, want 200; body=%s", listRecorder.Code, listRecorder.Body.String())
	}
}

func TestGetDoesNotConsumeRunScope(t *testing.T) {
	limiter := middleware.NewRateLimiter(1)
	engine := coverageEngine(t, "process_engineer", limiter)
	runRequest := httptest.NewRequest(http.MethodPost, "/api/v1/coverage-evaluations", strings.NewReader(`{"scenario_id":1}`))
	runRequest.Header.Set("Content-Type", "application/json")
	runRequest.Header.Set("Idempotency-Key", "run-get-0001")
	runRecorder := httptest.NewRecorder()
	engine.ServeHTTP(runRecorder, runRequest)
	if runRecorder.Code != http.StatusCreated {
		t.Fatalf("run status = %d, want 201", runRecorder.Code)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/coverage-evaluations/1", nil)
	getRecorder := httptest.NewRecorder()
	engine.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get after run status = %d, want 200; body=%s", getRecorder.Code, getRecorder.Body.String())
	}
}

func TestConfirmStillRateLimited(t *testing.T) {
	limiter := middleware.NewRateLimiter(1)
	engine := coverageEngine(t, "safety_reviewer", limiter)
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/coverage-evaluations/1/confirm", nil)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if i == 1 && recorder.Code != http.StatusTooManyRequests {
			t.Fatalf("second confirm status = %d, want 429; body=%s", recorder.Code, recorder.Body.String())
		}
	}
}

func coverageEngine(t *testing.T, role string, limiter *middleware.RateLimiter) *gin.Engine {
	t.Helper()
	coverageHandler := handler.NewCoverageEvaluationHandler(&stubCoverageService{})
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("actor", util.Actor{UserID: 1, Username: "user", Role: role, RequestID: "req-rate"})
		c.Next()
	})
	api := engine.Group("/api/v1")
	router.RegisterCoverageEvaluationRoutes(api, coverageHandler, limiter)
	return engine
}

type stubCoverageService struct{}

func (s *stubCoverageService) Run(ctx context.Context, request dto.RunCoverageEvaluationRequest, key string, actor util.Actor) (dto.CoverageEvaluationResponse, bool, error) {
	return dto.CoverageEvaluationResponse{ID: 1, ScenarioID: request.ScenarioID}, false, nil
}

func (s *stubCoverageService) Get(ctx context.Context, id uint) (dto.CoverageEvaluationResponse, error) {
	return dto.CoverageEvaluationResponse{ID: id}, nil
}

func (s *stubCoverageService) List(ctx context.Context, query dto.CoverageEvaluationQuery) (dto.CoverageEvaluationListResponse, error) {
	return dto.CoverageEvaluationListResponse{Page: query.Page, Size: query.PageSize}, nil
}

func (s *stubCoverageService) Confirm(ctx context.Context, id uint, actor util.Actor) (dto.CoverageEvaluationResponse, error) {
	return dto.CoverageEvaluationResponse{ID: id}, nil
}

func (s *stubCoverageService) Void(ctx context.Context, id uint, actor util.Actor) (dto.CoverageEvaluationResponse, error) {
	return dto.CoverageEvaluationResponse{ID: id}, nil
}

func (s *stubCoverageService) Replay(ctx context.Context, id uint, actor util.Actor) (dto.CoverageEvaluationResponse, error) {
	return dto.CoverageEvaluationResponse{ID: id}, nil
}

func (s *stubCoverageService) Compare(ctx context.Context, id uint, otherID uint) (dto.EvaluationComparisonResponse, error) {
	return dto.EvaluationComparisonResponse{BaseID: id, ComparedID: otherID}, nil
}
