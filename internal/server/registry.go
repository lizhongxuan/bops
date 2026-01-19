package server

import (
	"bops/internal/modules"
	"bops/internal/modules/cmd"
	"bops/internal/modules/envset"
	"bops/internal/modules/pkg"
	"bops/internal/modules/script"
	"bops/internal/modules/service"
	"bops/internal/modules/template"
	"bops/internal/scriptstore"
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
