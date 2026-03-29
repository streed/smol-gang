package scheduler

import (
	"testing"

	"github.com/streed/smol-gang/gateway/internal/models"
)

func TestValidateDAG_Valid(t *testing.T) {
	tasks := []models.TaskNode{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b", "c"}},
	}
	if err := ValidateDAG(tasks); err != nil {
		t.Fatalf("expected valid DAG, got: %v", err)
	}
}

func TestValidateDAG_Cycle(t *testing.T) {
	tasks := []models.TaskNode{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}
	err := ValidateDAG(tasks)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidateDAG_DuplicateID(t *testing.T) {
	tasks := []models.TaskNode{
		{ID: "a", DependsOn: []string{}},
		{ID: "a", DependsOn: []string{}},
	}
	err := ValidateDAG(tasks)
	if err == nil {
		t.Fatal("expected duplicate ID error")
	}
}

func TestValidateDAG_UnknownDep(t *testing.T) {
	tasks := []models.TaskNode{
		{ID: "a", DependsOn: []string{"z"}},
	}
	err := ValidateDAG(tasks)
	if err == nil {
		t.Fatal("expected unknown dep error")
	}
}

func TestComputeWaves(t *testing.T) {
	tasks := []models.TaskNode{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b", "c"}},
	}
	waves, err := ComputeWaves(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves, got %d", len(waves))
	}
	if waves[0][0] != "a" {
		t.Fatalf("expected wave 0 = [a], got %v", waves[0])
	}
	if len(waves[1]) != 2 {
		t.Fatalf("expected wave 1 has 2 tasks, got %d", len(waves[1]))
	}
	if waves[2][0] != "d" {
		t.Fatalf("expected wave 2 = [d], got %v", waves[2])
	}
}

func TestComputeWaves_SingleTask(t *testing.T) {
	tasks := []models.TaskNode{
		{ID: "main", DependsOn: []string{}},
	}
	waves, err := ComputeWaves(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 1 || waves[0][0] != "main" {
		t.Fatalf("expected 1 wave with 'main', got %v", waves)
	}
}

func TestComputeCriticalPath(t *testing.T) {
	tasks := []models.TaskNode{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b"}},
	}
	path := ComputeCriticalPath(tasks)
	// Critical path: a -> b -> d (length 3)
	if len(path) != 3 {
		t.Fatalf("expected critical path length 3, got %d: %v", len(path), path)
	}
}

func TestBuildExecutionPlan(t *testing.T) {
	tasks := []models.TaskNode{
		{ID: "schema", Description: "Create DB schema", DependsOn: []string{}},
		{ID: "model", Description: "Build user model", DependsOn: []string{"schema"}},
		{ID: "api", Description: "Create API endpoints", DependsOn: []string{"model"}},
		{ID: "tests", Description: "Write tests", DependsOn: []string{"model"}},
	}
	plan, err := BuildExecutionPlan(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Waves) != 3 {
		t.Fatalf("expected 3 waves, got %d", len(plan.Waves))
	}
	if len(plan.CriticalPath) < 3 {
		t.Fatalf("expected critical path >= 3, got %d", len(plan.CriticalPath))
	}
}
