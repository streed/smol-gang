package scheduler

import (
	"fmt"
	"sort"

	"github.com/streed/smol-gang/gateway/internal/models"
)

// ValidateDAG checks that the task list forms a valid DAG:
// - No duplicate IDs
// - All depends_on references point to existing tasks
// - No cycles (via Kahn's algorithm)
// - No orphans (unless they're root nodes with no deps)
func ValidateDAG(tasks []models.TaskNode) error {
	ids := make(map[string]bool)
	for _, t := range tasks {
		if ids[t.ID] {
			return fmt.Errorf("duplicate task ID: %s", t.ID)
		}
		ids[t.ID] = true
	}

	for _, t := range tasks {
		for _, dep := range t.DependsOn {
			if !ids[dep] {
				return fmt.Errorf("task %q depends on unknown task %q", t.ID, dep)
			}
			if dep == t.ID {
				return fmt.Errorf("task %q depends on itself", t.ID)
			}
		}
	}

	// Cycle detection via Kahn's algorithm
	inDegree := make(map[string]int)
	edges := make(map[string][]string) // parent -> children
	for _, t := range tasks {
		if _, ok := inDegree[t.ID]; !ok {
			inDegree[t.ID] = 0
		}
		for _, dep := range t.DependsOn {
			edges[dep] = append(edges[dep], t.ID)
			inDegree[t.ID]++
		}
	}

	queue := []string{}
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++
		for _, child := range edges[node] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	if visited != len(tasks) {
		return fmt.Errorf("cycle detected: %d of %d tasks are reachable", visited, len(tasks))
	}

	return nil
}

// ComputeWaves groups tasks into execution waves by BFS depth.
// Wave 0: tasks with no dependencies. Wave N: tasks whose deps are all in waves < N.
func ComputeWaves(tasks []models.TaskNode) ([][]string, error) {
	if err := ValidateDAG(tasks); err != nil {
		return nil, err
	}

	taskMap := make(map[string]*models.TaskNode)
	inDegree := make(map[string]int)
	edges := make(map[string][]string)

	for i := range tasks {
		t := &tasks[i]
		taskMap[t.ID] = t
		inDegree[t.ID] = len(t.DependsOn)
		for _, dep := range t.DependsOn {
			edges[dep] = append(edges[dep], t.ID)
		}
	}

	var waves [][]string
	remaining := len(tasks)

	for remaining > 0 {
		var wave []string
		for id, deg := range inDegree {
			if deg == 0 {
				wave = append(wave, id)
			}
		}
		if len(wave) == 0 {
			return nil, fmt.Errorf("cycle detected: %d nodes unreachable", remaining)
		}

		sort.Strings(wave) // deterministic ordering

		for _, id := range wave {
			delete(inDegree, id)
			for _, child := range edges[id] {
				inDegree[child]--
			}
		}

		waves = append(waves, wave)
		remaining -= len(wave)
	}

	return waves, nil
}

// ComputeCriticalPath finds the longest chain of sequential dependencies.
func ComputeCriticalPath(tasks []models.TaskNode) []string {
	taskMap := make(map[string]*models.TaskNode)
	edges := make(map[string][]string) // parent -> children
	for i := range tasks {
		taskMap[tasks[i].ID] = &tasks[i]
		for _, dep := range tasks[i].DependsOn {
			edges[dep] = append(edges[dep], tasks[i].ID)
		}
	}

	// Find longest path using DFS + memoization
	memo := make(map[string][]string)
	var longest func(id string) []string
	longest = func(id string) []string {
		if cached, ok := memo[id]; ok {
			return cached
		}
		best := []string{}
		for _, child := range edges[id] {
			path := longest(child)
			if len(path) > len(best) {
				best = path
			}
		}
		result := append([]string{id}, best...)
		memo[id] = result
		return result
	}

	// Find roots (no dependencies)
	var roots []string
	for _, t := range tasks {
		if len(t.DependsOn) == 0 {
			roots = append(roots, t.ID)
		}
	}

	var criticalPath []string
	for _, root := range roots {
		path := longest(root)
		if len(path) > len(criticalPath) {
			criticalPath = path
		}
	}

	return criticalPath
}

// BuildExecutionPlan validates, computes waves, and builds the full plan.
func BuildExecutionPlan(tasks []models.TaskNode) (*models.ExecutionPlan, error) {
	if len(tasks) == 0 {
		return nil, fmt.Errorf("empty task list")
	}

	waves, err := ComputeWaves(tasks)
	if err != nil {
		return nil, err
	}

	criticalPath := ComputeCriticalPath(tasks)

	return &models.ExecutionPlan{
		Tasks:        tasks,
		Waves:        waves,
		CriticalPath: criticalPath,
	}, nil
}

// AssignWaveNumbers returns a map of task ID -> wave number.
func AssignWaveNumbers(waves [][]string) map[string]int {
	result := make(map[string]int)
	for i, wave := range waves {
		for _, id := range wave {
			result[id] = i
		}
	}
	return result
}
