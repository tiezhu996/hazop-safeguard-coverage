package hzbq002

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"hazop-safeguard-coverage/backend/internal/dto"
	"hazop-safeguard-coverage/backend/internal/model"
	"hazop-safeguard-coverage/backend/internal/repository"
	"hazop-safeguard-coverage/backend/internal/service"
	"hazop-safeguard-coverage/backend/internal/util"
)

func TestMissingSafeguardReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := safeguardTestService(t)
	_, err := svc.Get(ctx, 999)
	assertStatus(t, err, http.StatusNotFound)
}

func TestFutureVerificationRejected(t *testing.T) {
	ctx := context.Background()
	svc, db := safeguardTestService(t)
	scenario := createScenarioForSafeguard(t, db)
	safeguard := model.Safeguard{
		Name: "Pressure alarm", SafeguardType: "alarm", TargetScenarioID: scenario.ID,
		IndependenceKey: "ALM-P-01", Effectiveness: 0.7, TestIntervalDays: 180,
		LifecycleState: "pending", EvidenceNote: "awaiting proof test",
	}
	if err := repository.NewSafeguardRepository(db).Create(ctx, &safeguard); err != nil {
		t.Fatalf("create safeguard: %v", err)
	}
	_, err := svc.Verify(ctx, safeguard.ID, dto.VerifySafeguardRequest{
		VerifiedAt: time.Now().Add(10 * time.Minute), EvidenceNote: "future proof test",
	}, util.Actor{UserID: 10, Username: "reviewer", Role: "safety_reviewer"})
	assertStatus(t, err, http.StatusUnprocessableEntity)
}

func TestLifecycleTransitionRequiresFromState(t *testing.T) {
	ctx := context.Background()
	svc, db := safeguardTestService(t)
	scenario := createScenarioForSafeguard(t, db)
	now := time.Now().Add(-10 * 24 * time.Hour)
	safeguard := model.Safeguard{
		Name: "Relief valve", SafeguardType: "relief", TargetScenarioID: scenario.ID,
		IndependenceKey: "PSV-01", Effectiveness: 0.9, TestIntervalDays: 365,
		LastVerifiedAt: &now, LifecycleState: "active", EvidenceNote: "certificate ok",
	}
	if err := repository.NewSafeguardRepository(db).Create(ctx, &safeguard); err != nil {
		t.Fatalf("create safeguard: %v", err)
	}
	_, err := svc.Restore(ctx, safeguard.ID, dto.SafeguardActionRequest{Reason: "not actually invalid"}, util.Actor{
		UserID: 10, Username: "reviewer", Role: "safety_reviewer",
	})
	assertStatus(t, err, http.StatusConflict)
}

func safeguardTestService(t *testing.T) (service.SafeguardService, *gorm.DB) {
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
	safeguardRepo := repository.NewSafeguardRepository(db)
	scenarioRepo := repository.NewDeviationScenarioRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	return service.NewSafeguardService(safeguardRepo, scenarioRepo, auditRepo), db
}

func createScenarioForSafeguard(t *testing.T, db *gorm.DB) model.DeviationScenario {
	t.Helper()
	node := model.ProcessNode{
		NodeCode: "R-1", Name: "Reactor", UnitName: "Unit", Medium: "gas",
		DesignPressure: 1, DesignTemperature: 100, OwnerTeam: "ops", Status: "active",
	}
	if err := repository.NewProcessNodeRepository(db).Create(context.Background(), &node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	scenario := model.DeviationScenario{
		ProcessNodeID: node.ID, Guideword: "more", Parameter: "pressure",
		Cause: "blocked outlet", Consequence: "rupture", Likelihood: 3, Severity: 5,
		ScenarioState: "draft", Version: 1, CreatedBy: 1, CreatedByName: "engineer",
	}
	if err := repository.NewDeviationScenarioRepository(db).Create(context.Background(), &scenario); err != nil {
		t.Fatalf("create scenario: %v", err)
	}
	return scenario
}

func assertStatus(t *testing.T, err error, want int) {
	t.Helper()
	var appErr *util.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %v", err)
	}
	if appErr.Status != want {
		t.Fatalf("status = %d, want %d; err=%v", appErr.Status, want, err)
	}
}
