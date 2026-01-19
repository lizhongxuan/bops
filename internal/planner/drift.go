package planner

import "bops/internal/state"

type DriftReport struct {
	ResourceID string               `json:"resource_id"`
	Diff       map[string]DiffEntry `json:"diff"`
	HasDrift   bool                 `json:"has_drift"`
}

func DetectDrift(resource state.ResourceState) DriftReport {
	diff := Diff(resource.Desired, resource.Current)
	return DriftReport{
		ResourceID: resource.ID,
		Diff:       diff,
		HasDrift:   len(diff) > 0,
	}
}
