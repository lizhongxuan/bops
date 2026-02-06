package workflow

import (
	"os"

	"gopkg.in/yaml.v3"
)

func LoadFile(path string) (Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Workflow{}, err
	}
	return Load(data)
}

func Load(data []byte) (Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return Workflow{}, err
	}
	normalizeWorkflow(&wf)
	return wf, nil
}

func normalizeWorkflow(wf *Workflow) {
	if wf == nil {
		return
	}
	for i := range wf.Steps {
		if len(wf.Steps[i].Args) == 0 && len(wf.Steps[i].With) > 0 {
			wf.Steps[i].Args = wf.Steps[i].With
		}
	}
	for i := range wf.Handlers {
		if len(wf.Handlers[i].Args) == 0 && len(wf.Handlers[i].With) > 0 {
			wf.Handlers[i].Args = wf.Handlers[i].With
		}
	}
	for i := range wf.Tests {
		if len(wf.Tests[i].Args) == 0 && len(wf.Tests[i].With) > 0 {
			wf.Tests[i].Args = wf.Tests[i].With
		}
	}
}
