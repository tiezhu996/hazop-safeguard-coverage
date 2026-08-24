package hzds004

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"hazop-safeguard-coverage/backend/internal/algorithm"
	"hazop-safeguard-coverage/backend/internal/dto"
	"hazop-safeguard-coverage/backend/internal/model"
	"hazop-safeguard-coverage/backend/internal/repository"
	"hazop-safeguard-coverage/backend/internal/service"
	"hazop-safeguard-coverage/backend/internal/util"
)

func TestNilVerificationRejectedNoPanic(t *testing.T) {
	ref := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	snapshot := algorithm.NewSnapshot(
		model.ProcessNode{ID: 1},
		model.DeviationScenario{ID: 1, Cause: "cooling loss", Consequence: "overpressure"},
		[]model.Safeguard{{ID: 1, IndependenceKey: "SIS", Effectiveness: 0.8, TestIntervalDays: 30, LifecycleState: "active"}},
		ref,
	)
	result, err := algorithm.NewEvaluator().Evaluate(snapshot)
	if err != nil {
		t.Fatalf("evaluate with nil verification returned error: %v", err)
	}
	if len(result.Explanation.Deduplicated) != 0 {
		t.Fatalf("expected no retained safeguard, got %#v", result.Explanation.Deduplicated)
	}
}

func TestSafeguardResponseNilVerificationNoPanic(t *testing.T) {
	now := time.Now().UTC()
	safeguard := model.Safeguard{
		ID: 1, Name: "Pending alarm", SafeguardType: "alarm", IndependenceKey: "ALM",
		Effectiveness: 0.7, TestIntervalDays: 180, LifecycleState: "pending", EvidenceNote: "no proof yet",
	}
	response := dto.NewSafeguardResponse(safeguard, now)
	if !response.VerificationExpired {
		t.Fatal("pending safeguard with no verification should be marked expired")
	}
}

func TestSafeguardVerificationExpiresNilNoPanic(t *testing.T) {
	safeguard := model.Safeguard{LastVerifiedAt: nil, TestIntervalDays: 180}
	if expires := safeguard.VerificationExpiresAt(); expires != nil {
		t.Fatalf("nil verification should not produce expiry, got %v", expires)
	}
}

func TestRestoreInvalidNilVerificationNoPanic(t *testing.T) {
	ctx := context.Background()
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
	node := model.ProcessNode{NodeCode: "R-1", Name: "Reactor", UnitName: "Unit", Medium: "gas", DesignPressure: 1, DesignTemperature: 100, OwnerTeam: "ops", Status: "active"}
	if err := repository.NewProcessNodeRepository(db).Create(ctx, &node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	scenario := model.DeviationScenario{ProcessNodeID: node.ID, Guideword: "more", Parameter: "pressure", Cause: "blocked", Consequence: "rupture", Likelihood: 3, Severity: 5, ScenarioState: "draft", Version: 1}
	if err := repository.NewDeviationScenarioRepository(db).Create(ctx, &scenario); err != nil {
		t.Fatalf("create scenario: %v", err)
	}
	safeguard := model.Safeguard{
		Name: "Relief", SafeguardType: "relief", TargetScenarioID: scenario.ID,
		IndependenceKey: "PSV", Effectiveness: 0.9, TestIntervalDays: 365,
		LifecycleState: "invalid", EvidenceNote: "voided",
	}
	if err := repository.NewSafeguardRepository(db).Create(ctx, &safeguard); err != nil {
		t.Fatalf("create safeguard: %v", err)
	}
	svc := service.NewSafeguardService(
		repository.NewSafeguardRepository(db),
		repository.NewDeviationScenarioRepository(db),
		repository.NewAuditRepository(db),
	)
	result, err := svc.Restore(ctx, safeguard.ID, dto.SafeguardActionRequest{Reason: "restore for revalidation"}, util.Actor{
		UserID: 10, Username: "reviewer", Role: "safety_reviewer", RequestID: "req-004",
	})
	if err != nil {
		t.Fatalf("restore returned error: %v", err)
	}
	if result.LifecycleState != "pending" {
		t.Fatalf("state = %s, want pending", result.LifecycleState)
	}
}
