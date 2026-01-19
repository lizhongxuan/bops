package tester

import "context"

type CompatibilityResult struct {
	Image string `json:"image"`
	Result Result `json:"result"`
	Error  string `json:"error,omitempty"`
}

func RunCompatibility(ctx context.Context, runner *DockerRunner, images []string, args []string) []CompatibilityResult {
	results := make([]CompatibilityResult, 0, len(images))
	for _, image := range images {
		res, err := runner.StartAgent(ctx, image, args)
		item := CompatibilityResult{Image: image, Result: res}
		if err != nil {
			item.Error = err.Error()
		}
		results = append(results, item)
	}
	return results
}
