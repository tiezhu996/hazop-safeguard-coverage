package hzjy010

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"hazop-safeguard-coverage/backend/internal/algorithm"
	"hazop-safeguard-coverage/backend/internal/handler"
	"hazop-safeguard-coverage/backend/internal/middleware"
	"hazop-safeguard-coverage/backend/internal/model"
	"hazop-safeguard-coverage/backend/internal/repository"
	"hazop-safeguard-coverage/backend/internal/router"
	"hazop-safeguard-coverage/backend/internal/service"
	"hazop-safeguard-coverage/backend/internal/util"
)

func TestMissingEvaluationReturnsNotFound(t *testing.T) {
	engine := evaluationRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/coverage-evaluations/999", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET missing evaluation status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMissingEvaluationReplayReturnsNotFound(t *testing.T) {
	engine := evaluationRouter(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coverage-evaluations/999/replay", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("replay missing evaluation status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMissingScenarioRunReturnsNotFound(t *testing.T) {
	engine := evaluationRouter(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coverage-evaluations", strings.NewReader(`{"scenario_id":999}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "run-missing-999")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("run missing scenario status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
	}
}

func evaluationRouter(t *testing.T) *gin.Engine {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.ProcessNode{}, &model.DeviationScenario{},
		&model.Safeguard{}, &model.CoverageEvaluation{}, &model.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	evaluationRepo := repository.NewCoverageEvaluationRepository(db)
	scenarioRepo := repository.NewDeviationScenarioRepository(db)
	nodeRepo := repository.NewProcessNodeRepository(db)
	safeguardRepo := repository.NewSafeguardRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	evaluationService := service.NewCoverageEvaluationService(
		evaluationRepo, scenarioRepo, nodeRepo, safeguardRepo, auditRepo, algorithm.NewEvaluator(),
	)
	evaluationHandler := handler.NewCoverageEvaluationHandler(evaluationService)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("actor", util.Actor{UserID: 1, Username: "engineer", Role: "process_engineer", RequestID: "req-coverage"})
		c.Next()
	})
	api := engine.Group("/api/v1")
	router.RegisterCoverageEvaluationRoutes(api, evaluationHandler, middleware.NewRateLimiter(100))
	return engine
}
