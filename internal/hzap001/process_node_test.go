package hzap001

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"hazop-safeguard-coverage/backend/internal/handler"
	"hazop-safeguard-coverage/backend/internal/model"
	"hazop-safeguard-coverage/backend/internal/repository"
	"hazop-safeguard-coverage/backend/internal/service"
)

func TestMissingNodeDetailReturnsNotFound(t *testing.T) {
	assertNodeStatus(t, http.MethodGet, "/process-nodes/999", "", http.StatusNotFound)
}

func TestMissingNodeUpdateReturnsNotFound(t *testing.T) {
	assertNodeStatus(t, http.MethodPut, "/process-nodes/999", `{}`, http.StatusNotFound)
}

func TestMissingNodeDeactivateReturnsNotFound(t *testing.T) {
	assertNodeStatus(t, http.MethodPost, "/process-nodes/999/deactivate", "", http.StatusNotFound)
}

func assertNodeStatus(t *testing.T, method, path, body string, want int) {
	t.Helper()
	router := nodeRouter(t)
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != want {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, recorder.Code, want, recorder.Body.String())
	}
}

func nodeRouter(t *testing.T) *gin.Engine {
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
	nodeRepo := repository.NewProcessNodeRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	processNodeHandler := handler.NewProcessNodeHandler(service.NewProcessNodeService(nodeRepo, auditRepo))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/process-nodes/:id", processNodeHandler.Get)
	router.PUT("/process-nodes/:id", processNodeHandler.Update)
	router.POST("/process-nodes/:id/deactivate", processNodeHandler.Deactivate)
	return router
}
