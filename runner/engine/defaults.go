package engine

import (
	"bops/runner/modules"
	"bops/runner/modules/cmd"
	"bops/runner/modules/envset"
	"bops/runner/modules/script"
	"bops/runner/modules/shell"
	"bops/runner/modules/template"
	"bops/runner/modules/wait"
	"bops/runner/scriptstore"
)

// DefaultRegistry returns a registry populated with the built-in modules.
// Pass nil scriptStore to skip script.shell/script.python modules.
func DefaultRegistry(scriptStore *scriptstore.Store) *modules.Registry {
	reg := modules.NewRegistry()
	_ = reg.Register("cmd.run", cmd.New())
	_ = reg.Register("shell.run", shell.New())
	_ = reg.Register("env.set", envset.New())
	if scriptStore != nil {
		_ = reg.Register("script.shell", script.New("shell", scriptStore))
		_ = reg.Register("script.python", script.New("python", scriptStore))
	}
	_ = reg.Register("template.render", template.New())
	_ = reg.Register("wait.event", wait.NewEvent())
	return reg
}
