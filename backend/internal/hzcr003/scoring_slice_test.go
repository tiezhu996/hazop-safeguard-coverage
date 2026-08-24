package hzcr003

import (
	"reflect"
	"testing"
	"time"

	"hazop-safeguard-coverage/backend/internal/algorithm"
	"hazop-safeguard-coverage/backend/internal/model"
)

func TestNewSnapshotDoesNotMutateSafeguardOrder(t *testing.T) {
	ref := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	verified := ref.AddDate(0, 0, -10)
	original := []model.Safeguard{
		{ID: 2, IndependenceKey: "B", Effectiveness: 0.8, TestIntervalDays: 30, LastVerifiedAt: &verified, LifecycleState: "active"},
		{ID: 1, IndependenceKey: "A", Effectiveness: 0.6, TestIntervalDays: 30, LastVerifiedAt: &verified, LifecycleState: "active"},
	}
	expected := append([]model.Safeguard(nil), original...)
	_ = algorithm.NewSnapshot(model.ProcessNode{ID: 1}, model.DeviationScenario{ID: 1}, original, ref)
	if !reflect.DeepEqual(original, expected) {
		t.Fatalf("NewSnapshot mutated caller slice: got %#v, want %#v", original, expected)
	}
}

func TestCalculateScoreDoesNotMutateSafeguardOrder(t *testing.T) {
	ref := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	verified := ref.AddDate(0, 0, -10)
	original := []algorithm.SnapshotSafeguard{
		{ID: 2, IndependenceKey: "B", Effectiveness: 0.8, TestIntervalDays: 30, LastVerifiedAt: &verified, LifecycleState: "active"},
		{ID: 1, IndependenceKey: "A", Effectiveness: 0.6, TestIntervalDays: 30, LastVerifiedAt: &verified, LifecycleState: "active"},
	}
	expected := append([]algorithm.SnapshotSafeguard(nil), original...)
	_ = algorithm.CalculateScore(algorithm.Graph{}, algorithm.SnapshotScenario{}, original, nil)
	if !reflect.DeepEqual(original, expected) {
		t.Fatalf("CalculateScore mutated caller slice: got %#v, want %#v", original, expected)
	}
}

func TestGraphMitigatesFirstConsequence(t *testing.T) {
	ref := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	verified := ref.AddDate(0, 0, -10)
	snapshot := algorithm.NewSnapshot(
		model.ProcessNode{ID: 1, NodeCode: "R-1"},
		model.DeviationScenario{ID: 1, Cause: "cooling loss; blocked outlet", Consequence: "overpressure; release"},
		[]model.Safeguard{{ID: 1, IndependenceKey: "SIS", Effectiveness: 0.8, TestIntervalDays: 30, LastVerifiedAt: &verified, LifecycleState: "active"}},
		ref,
	)
	graph := algorithm.BuildGraph(snapshot)
	found := false
	for _, edge := range graph.Edges {
		if edge.From == "safeguard-1" && edge.To == "consequence-01" && edge.Relation == "mitigates" {
			found = true
		}
	}
	if !found {
		t.Fatal("first consequence has no safeguard edge")
	}
}

func TestGraphBuildsAllCauseConsequencePaths(t *testing.T) {
	ref := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	snapshot := algorithm.NewSnapshot(
		model.ProcessNode{ID: 1, NodeCode: "R-1"},
		model.DeviationScenario{ID: 1, Cause: "cooling loss; blocked outlet", Consequence: "overpressure; release"},
		[]model.Safeguard{},
		ref,
	)
	graph := algorithm.BuildGraph(snapshot)
	if len(graph.Paths) != 4 {
		t.Fatalf("paths = %d, want 4", len(graph.Paths))
	}
}
