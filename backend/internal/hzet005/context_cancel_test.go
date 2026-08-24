package hzet005

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"hazop-safeguard-coverage/backend/internal/dto"
	"hazop-safeguard-coverage/backend/internal/middleware"
	"hazop-safeguard-coverage/backend/internal/model"
	"hazop-safeguard-coverage/backend/internal/repository"
	"hazop-safeguard-coverage/backend/internal/util"
)

func TestCoverageRepoCreateRespectsCanceledContext(t *testing.T) {
	repo := coverageRepo(t)
	err := repo.Create(canceledContext(), newEvaluation(1, "key-create", "queued"))
	if err == nil {
		t.Fatal("Create succeeded with canceled context")
	}
}

func TestCoverageRepoGetRespectsCanceledContext(t *testing.T) {
	ctx := context.Background()
	repo := coverageRepo(t)
	evaluation := newEvaluation(1, "key-get", "queued")
	if err := repo.Create(ctx, evaluation); err != nil {
		t.Fatalf("seed evaluation: %v", err)
	}
	if _, err := repo.GetByID(canceledContext(), evaluation.ID); err == nil {
		t.Fatal("GetByID succeeded with canceled context")
	}
}

func TestCoverageRepoFindIdempotencyRespectsCanceledContext(t *testing.T) {
	ctx := context.Background()
	repo := coverageRepo(t)
	if err := repo.Create(ctx, newEvaluation(1, "key-find", "queued")); err != nil {
		t.Fatalf("seed evaluation: %v", err)
	}
	if _, err := repo.FindByIdempotencyKey(canceledContext(), "key-find"); err == nil {
		t.Fatal("FindByIdempotencyKey succeeded with canceled context")
	}
}

func TestCoverageRepoListRespectsCanceledContext(t *testing.T) {
	ctx := context.Background()
	repo := coverageRepo(t)
	if err := repo.Create(ctx, newEvaluation(1, "key-list", "queued")); err != nil {
		t.Fatalf("seed evaluation: %v", err)
	}
	if _, _, err := repo.List(canceledContext(), dto.CoverageEvaluationQuery{Page: 1, PageSize: 20}); err == nil {
		t.Fatal("List succeeded with canceled context")
	}
}

func TestCoverageRepoTransitionRespectsCanceledContext(t *testing.T) {
	ctx := context.Background()
	repo := coverageRepo(t)
	evaluation := newEvaluation(1, "key-transition", "queued")
	if err := repo.Create(ctx, evaluation); err != nil {
		t.Fatalf("seed evaluation: %v", err)
	}
	if _, err := repo.Transition(canceledContext(), evaluation.ID, []string{"queued"}, "running", nil); err == nil {
		t.Fatal("Transition succeeded with canceled context")
	}
}

func TestCoverageRepoCompleteRespectsCanceledContext(t *testing.T) {
	ctx := context.Background()
	repo := coverageRepo(t)
	evaluation := newEvaluation(1, "key-complete", "running")
	if err := repo.Create(ctx, evaluation); err != nil {
		t.Fatalf("seed evaluation: %v", err)
	}
	if _, err := repo.Complete(canceledContext(), evaluation.ID, "running", map[string]any{}); err == nil {
		t.Fatal("Complete succeeded with canceled context")
	}
}

func TestCoverageRepoReplayResultRespectsCanceledContext(t *testing.T) {
	ctx := context.Background()
	repo := coverageRepo(t)
	evaluation := newEvaluation(1, "key-replay", "completed")
	if err := repo.Create(ctx, evaluation); err != nil {
		t.Fatalf("seed evaluation: %v", err)
	}
	if err := repo.SetReplayResult(canceledContext(), evaluation.ID, true); err == nil {
		t.Fatal("SetReplayResult succeeded with canceled context")
	}
}

func TestAuditRecordRespectsCanceledContext(t *testing.T) {
	repo := auditRepo(t)
	if err := repo.Record(canceledContext(), auditLog("req-cancel")); err == nil {
		t.Fatal("Record succeeded with canceled context")
	}
}

func TestAuditListRespectsCanceledContext(t *testing.T) {
	ctx := context.Background()
	repo := auditRepo(t)
	if err := repo.Record(ctx, auditLog("req-list")); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	if _, _, err := repo.List(canceledContext(), repository.AuditQuery{Page: 1, PageSize: 20}); err == nil {
		t.Fatal("List succeeded with canceled context")
	}
}

func TestAuditMiddlewareStopsOnCanceledRequest(t *testing.T) {
	db := openDB(t)
	audits := repository.NewAuditRepository(db)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("actor", util.Actor{UserID: 1, Username: "engineer", Role: "process_engineer", RequestID: "req-mw"})
		c.Next()
	})
	router.Use(middleware.Audit(audits))
	router.POST("/demo", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/demo", nil).WithContext(canceled)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	var count int64
	if err := db.Model(&model.AuditLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if count != 0 {
		t.Fatalf("canceled request wrote %d audit logs, want 0", count)
	}
}

func TestAuditListHandlerStopsOnCanceledRequest(t *testing.T) {
	db := openDB(t)
	audits := repository.NewAuditRepository(db)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/audit-logs", middleware.AuditListHandler(audits))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/audit-logs", nil).WithContext(canceled)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("canceled audit list status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestUserFindRespectsCanceledContext(t *testing.T) {
	db := openDB(t)
	repo := repository.NewUserRepository(db)
	if err := db.Create(&model.User{Username: "u1", DisplayName: "User One", PasswordHash: "x", Role: "process_engineer", Active: true}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := repo.FindByUsername(canceledContext(), "u1"); err == nil {
		t.Fatal("FindByUsername succeeded with canceled context")
	}
}

func TestUserGetRespectsCanceledContext(t *testing.T) {
	db := openDB(t)
	repo := repository.NewUserRepository(db)
	user := model.User{Username: "u2", DisplayName: "User Two", PasswordHash: "x", Role: "process_engineer", Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := repo.GetByID(canceledContext(), user.ID); err == nil {
		t.Fatal("GetByID succeeded with canceled context")
	}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func coverageRepo(t *testing.T) repository.CoverageEvaluationRepository {
	t.Helper()
	return repository.NewCoverageEvaluationRepository(openDB(t))
}

func auditRepo(t *testing.T) repository.AuditRepository {
	t.Helper()
	return repository.NewAuditRepository(openDB(t))
}

func openDB(t *testing.T) *gorm.DB {
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
	return db
}

func newEvaluation(id uint, key, state string) *model.CoverageEvaluation {
	now := time.Now().UTC()
	return &model.CoverageEvaluation{
		ID: id, ScenarioID: 1, AlgorithmVersion: "hazop-cover-v1.0.0",
		InputSnapshot: "{}", InputHash: "hash", CoverageScore: 0,
		UncoveredPaths: "[]", DeduplicatedSafeguards: "[]", Explanation: "{}",
		RiskRankBefore: "low", RiskRankAfter: "low", EvaluationState: state,
		EvaluatedBy: 1, EvaluatedByName: "engineer", EvaluatedAt: now,
		CreatedAt: now, UpdatedAt: now, IdempotencyKey: key,
	}
}

func auditLog(requestID string) model.AuditLog {
	return model.AuditLog{
		RequestID: requestID, ActorID: 1, ActorName: "engineer", ActorRole: "process_engineer",
		EntityType: "process_node", EntityID: 1, Action: "create",
		BeforeSnapshot: "{}", AfterSnapshot: "{}", CreatedAt: time.Now().UTC(),
	}
}
