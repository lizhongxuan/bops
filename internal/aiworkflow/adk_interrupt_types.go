package aiworkflow

import "github.com/cloudwego/eino/schema"

type missingInfo struct {
	Missing []string `json:"missing"`
}

type pauseInfo struct {
	StepID   string `json:"step_id"`
	StepName string `json:"step_name"`
	Reason   string `json:"reason"`
}

func init() {
	schema.RegisterName[*missingInfo]("bops_missing_info")
	schema.RegisterName[*pauseInfo]("bops_pause_info")
}
