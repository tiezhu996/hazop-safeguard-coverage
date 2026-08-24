package hzhw008

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"hazop-safeguard-coverage/backend/internal/dto"
	"hazop-safeguard-coverage/backend/internal/model"
	"hazop-safeguard-coverage/backend/internal/repository"
	"hazop-safeguard-coverage/backend/internal/service"
	"hazop-safeguard-coverage/backend/internal/util"
)

func TestScenarioRejectsDraftToVerified(t *testing.T) {
	ctx := context.Background()
	svc, db := stateTestService(t)
	scenario := createStateScenario(t, db)
	reviewer := util.Actor{UserID: 20, Username: "reviewer", Role: "safety_reviewer", RequestID: "req-state-1"}
	_, err := svc.Transition(ctx, scenario.ID, dto.TransitionScenarioRequest{
		ToState: "verified", Version: scenario.Version,
	}, reviewer)
	assertAppStatus(t, err, http.StatusConflict)
}

func TestScenarioAcceptedRequiresReviewerSeparation(t *testing.T) {
	ctx := context.Background()
	svc, db := stateTestService(t)
	scenario := createStateScenario(t, db)
	engineer := util.Actor{UserID: 10, Username: "engineer", Role: "process_engineer", RequestID: "req-state-2"}
	reviewer := util.Actor{UserID: 20, Username: "reviewer", Role: "safety_reviewer", RequestID: "req-state-3"}
	analyzed, err := svc.Transition(ctx, scenario.ID, dto.TransitionScenarioRequest{
		ToState: "analyzed", Version: scenario.Version,
	}, engineer)
	if err != nil {
		t.Fatalf("analyze transition: %v", err)
	}
	verified, err := svc.Transition(ctx, scenario.ID, dto.TransitionScenarioRequest{
		ToState: "verified", Version: analyzed.Version,
	}, reviewer)
	if err != nil {
		t.Fatalf("verified transition: %v", err)
	}
	authorAsReviewer := util.Actor{UserID: 10, Username: "engineer", Role: "safety_reviewer", RequestID: "req-state-4"}
	_, err = svc.Transition(ctx, scenario.ID, dto.TransitionScenarioRequest{
		ToState: "accepted", Version: verified.Version,
	}, authorAsReviewer)
	assertAppStatus(t, err, http.StatusConflict)
}

func TestScenarioTransitionRequiresVersion(t *testing.T) {
	ctx := context.Background()
	svc, db := stateTestService(t)
	scenario := createStateScenario(t, db)
	engineer := util.Actor{UserID: 10, Username: "engineer", Role: "process_engineer", RequestID: "req-state-5"}
	reviewer := util.Actor{UserID: 20, Username: "reviewer", Role: "safety_reviewer", RequestID: "req-state-6"}
	analyzed, err := svc.Transition(ctx, scenario.ID, dto.TransitionScenarioRequest{
		ToState: "analyzed", Version: scenario.Version,
	}, engineer)
	if err != nil {
		t.Fatalf("analyze transition: %v", err)
	}
	staleVersion := analyzed.Version - 1
	_, err = svc.Transition(ctx, scenario.ID, dto.TransitionScenarioRequest{
		ToState: "verified", Version: staleVersion,
	}, reviewer)
	assertAppStatus(t, err, http.StatusConflict)
}

func TestScenarioRejectsReworkToAccepted(t *testing.T) {
	ctx := context.Background()
	svc, db := stateTestService(t)
	scenario := createStateScenario(t, db)
	engineer := util.Actor{UserID: 10, Username: "engineer", Role: "process_engineer", RequestID: "req-state-7"}
	reviewer := util.Actor{UserID: 20, Username: "reviewer", Role: "safety_reviewer", RequestID: "req-state-8"}
	analyzed, err := svc.Transition(ctx, scenario.ID, dto.TransitionScenarioRequest{
		ToState: "analyzed", Version: scenario.Version,
	}, engineer)
	if err != nil {
		t.Fatalf("analyze transition: %v", err)
	}
	rework, err := svc.Transition(ctx, scenario.ID, dto.TransitionScenarioRequest{
		ToState: "rework", Version: analyzed.Version,
	}, reviewer)
	if err != nil {
		t.Fatalf("rework transition: %v", err)
	}
	_, err = svc.Transition(ctx, scenario.ID, dto.TransitionScenarioRequest{
		ToState: "accepted", Version: rework.Version,
	}, reviewer)
	assertAppStatus(t, err, http.StatusConflict)
}

func TestScenarioUpdateRequiresVersion(t *testing.T) {
	ctx := context.Background()
	svc, db := stateTestService(t)
	scenario := createStateScenario(t, db)
	parameter := "pressure"
	_, err := svc.Update(ctx, scenario.ID, dto.UpdateDeviationScenarioRequest{
		Parameter: &parameter, Version: scenario.Version + 1,
	}, util.Actor{UserID: 10, Username: "engineer", Role: "process_engineer", RequestID: "req-state-9"})
	assertAppStatus(t, err, http.StatusConflict)
}

func stateTestService(t *testing.T) (service.DeviationScenarioService, *gorm.DB) {
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
	return service.NewDeviationScenarioService(
		repository.NewDeviationScenarioRepository(db),
		repository.NewProcessNodeRepository(db),
		repository.NewAuditRepository(db),
	), db
}

func createStateScenario(t *testing.T, db *gorm.DB) model.DeviationScenario {
	t.Helper()
	node := model.ProcessNode{
		NodeCode: "R-1", Name: "Reactor", UnitName: "Unit", Medium: "gas",
		DesignPressure: 1, DesignTemperature: 100, OwnerTeam: "ops", Status: "active",
	}
	if err := repository.NewProcessNodeRepository(db).Create(context.Background(), &node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	scenario := model.DeviationScenario{
		ProcessNodeID: node.ID, Guideword: "more", Parameter: "temperature",
		Cause: "cooling loss", Consequence: "overpressure", Likelihood: 4, Severity: 5,
		ScenarioState: "draft", Version: 1, CreatedBy: 10, CreatedByName: "engineer",
	}
	if err := repository.NewDeviationScenarioRepository(db).Create(context.Background(), &scenario); err != nil {
		t.Fatalf("create scenario: %v", err)
	}
	return scenario
}

func assertAppStatus(t *testing.T, err error, want int) {
	t.Helper()
	var appErr *util.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %v", err)
	}
	if appErr.Status != want {
		t.Fatalf("status = %d, want %d; err=%v", appErr.Status, want, err)
	}
}
