package aiworkflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

func parseStepPatchJSON(reply string) (StepPatch, error) {
	trimmed := strings.TrimSpace(reply)
	jsonText := extractJSONBlock(trimmed)
	if jsonText == "" {
		return StepPatch{}, fmt.Errorf("step patch response is not json")
	}
	var patch StepPatch
	decoder := json.NewDecoder(strings.NewReader(jsonText))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&patch); err != nil {
		return StepPatch{}, err
	}
	return normalizeStepPatch(patch)
}

func normalizeStepPatch(patch StepPatch) (StepPatch, error) {
	patch.StepName = strings.TrimSpace(patch.StepName)
	patch.StepID = strings.TrimSpace(patch.StepID)
	patch.Action = strings.TrimSpace(patch.Action)
	if patch.StepName == "" {
		return StepPatch{}, fmt.Errorf("step_name is required")
	}
	if patch.StepID == "" {
		patch.StepID = normalizePlanID(patch.StepName, 0)
	}
	if patch.Action == "" {
		return StepPatch{}, fmt.Errorf("action is required")
	}
	if !isAllowedAction(patch.Action) {
		return StepPatch{}, fmt.Errorf("action %q is not allowed", patch.Action)
	}
	if patch.Args == nil {
		patch.Args = map[string]any{}
	}
	if patch.Summary == "" {
		patch.Summary = patch.StepName
		if patch.Action != "" {
			patch.Summary = fmt.Sprintf("%s · %s", patch.StepName, patch.Action)
		}
	}
	return patch, nil
}

func alignStepPatchWithPlan(store *DraftStore, draftID string, patch StepPatch) StepPatch {
	if store == nil {
		return patch
	}
	snapshot := store.Snapshot(draftID)
	if snapshot.DraftID == "" || len(snapshot.Plan) == 0 {
		return patch
	}
	if patch.StepID != "" {
		for _, step := range snapshot.Plan {
			if step.ID == patch.StepID {
				return patch
			}
		}
	}
	for _, step := range snapshot.Plan {
		if strings.EqualFold(step.StepName, patch.StepName) {
			patch.StepID = step.ID
			if patch.StepName == "" {
				patch.StepName = step.StepName
			}
			return patch
		}
	}
	return patch
}

func validateStepPatch(patch StepPatch) []string {
	issues := []string{}
	if strings.TrimSpace(patch.StepName) == "" {
		issues = append(issues, "step_name is required")
	}
	if strings.TrimSpace(patch.Action) == "" {
		issues = append(issues, "action is required")
	} else if !isAllowedAction(strings.TrimSpace(patch.Action)) {
		issues = append(issues, fmt.Sprintf("action %q is not allowed", patch.Action))
	}
	return issues
}
