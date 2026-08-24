package hzix009

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

func TestUpdateNodeEmptyBodyNoPanic(t *testing.T) {
	assertUpdateStatus(t, `{}`, http.StatusNotFound)
}

func TestUpdateNodeMalformedJSONReturnsBadRequest(t *testing.T) {
	assertUpdateStatus(t, `{`, http.StatusBadRequest)
}

func TestCreateNodeMalformedJSONReturnsBadRequest(t *testing.T) {
	router := updateRouter(t)
	request := httptest.NewRequest(http.MethodPost, "/process-nodes", strings.NewReader(`{`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST malformed status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestNodeUnknownStatusNotActive(t *testing.T) {
	node := model.ProcessNode{Status: "disabled"}
	if node.Active() {
		t.Fatal("unknown status must not be treated as active")
	}
}


func assertUpdateStatus(t *testing.T, body string, want int) {
	t.Helper()
	router := updateRouter(t)
	request := httptest.NewRequest(http.MethodPut, "/process-nodes/999", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != want {
		t.Fatalf("PUT status = %d, want %d; body=%s", recorder.Code, want, recorder.Body.String())
	}
}

func updateRouter(t *testing.T) *gin.Engine {
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
	nodeHandler := handler.NewProcessNodeHandler(service.NewProcessNodeService(
		repository.NewProcessNodeRepository(db),
		repository.NewAuditRepository(db),
	))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/process-nodes/:id", nodeHandler.Update)
	router.POST("/process-nodes", nodeHandler.Create)
	return router
}
