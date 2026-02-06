package server

import (
	"bops/runner/modules"
	"bops/runner/modules/cmd"
	"bops/runner/modules/envset"
	"bops/runner/modules/pkg"
	"bops/runner/modules/script"
	"bops/runner/modules/service"
	"bops/runner/modules/template"
	"bops/runner/scriptstore"
)

func defaultRegistry(scriptStore *scriptstore.Store) *modules.Registry {
	reg := modules.NewRegistry()
	_ = reg.Register("cmd.run", cmd.New())
	_ = reg.Register("env.set", envset.New())
	_ = reg.Register("pkg.install", pkg.New())
	_ = reg.Register("script.shell", script.New("shell", scriptStore))
	_ = reg.Register("script.python", script.New("python", scriptStore))
	_ = reg.Register("template.render", template.New())
	_ = reg.Register("service.ensure", service.New())
	_ = reg.Register("service.restart", service.New())
	return reg
}
