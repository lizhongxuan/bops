package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"bops/internal/config"
	"bops/internal/engine"
	"bops/internal/logging"
	"bops/internal/modules"
	"bops/internal/modules/cmd"
	"bops/internal/modules/envset"
	"bops/internal/modules/pkg"
	"bops/internal/modules/script"
	"bops/internal/modules/service"
	"bops/internal/modules/template"
	"bops/internal/report"
	"bops/internal/scriptstore"
	"bops/internal/server"
	"bops/internal/state"
	"bops/internal/workflow"
	"go.uber.org/zap"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	_, _ = logging.Init(config.DefaultConfig())

	switch os.Args[1] {
	case "plan":
		if err := runPlan(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "apply":
		if err := runApply(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "test":
		if err := runTest(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "status":
		if err := runStatus(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "serve":
		if err := runServe(os.Args[2:]); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func runPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	file := fs.String("f", "", "workflow file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("workflow file is required")
	}

	logging.L().Debug("plan start", zap.String("file", *file))
	wf, err := loadWorkflow(*file)
	if err != nil {
		return err
	}

	eng := engine.New(defaultRegistry())
	plan, err := eng.Plan(context.Background(), wf)
	if err != nil {
		return err
	}

	return printJSON(plan)
}

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	file := fs.String("f", "", "workflow file")
	verbose := fs.Bool("verbose", false, "print step output")
	verboseShort := fs.Bool("v", false, "print step output (shorthand)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("workflow file is required")
	}

	logging.L().Debug("apply start", zap.String("file", *file))
	wf, err := loadWorkflow(*file)
	if err != nil {
		return err
	}

	eng := engine.New(defaultRegistry())
	if *verbose || *verboseShort {
		eng.Verbose = true
		eng.Out = os.Stdout
	}
	return eng.Apply(context.Background(), wf)
}

func runTest(args []string) error {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	file := fs.String("f", "", "workflow file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("workflow file is required")
	}

	logging.L().Debug("test plan start", zap.String("file", *file))
	wf, err := loadWorkflow(*file)
	if err != nil {
		return err
	}

	eng := engine.New(defaultRegistry())
	plan, err := eng.Plan(context.Background(), wf)
	if err != nil {
		return err
	}
	return printJSON(plan)
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	_, _ = logging.Init(cfg)
	logging.L().Debug("status requested")
	store := state.NewFileStore(cfg.StatePath)
	data, err := store.Load()
	if err != nil {
		return err
	}
	if len(data.Runs) == 0 {
		fmt.Println("no runs")
		return nil
	}

	summary := report.Summarize(data.Runs[len(data.Runs)-1])
	return printJSON(summary)
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", "", "config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	resolvedPath := config.ResolvePath(*configPath)
	cfg, err := config.Load(resolvedPath)
	if err != nil {
		return err
	}
	_, _ = logging.Init(cfg)
	logging.L().Info("server starting",
		zap.String("listen", cfg.ServerListen),
		zap.String("static_dir", cfg.StaticDir),
		zap.Strings("cors_origins", cfg.CORSOrigins),
		zap.String("ai_provider", cfg.AIProvider),
		zap.String("ai_model", cfg.AIModel),
		zap.String("config_path", resolvedPath),
	)

	srv := server.New(cfg, resolvedPath)
	return srv.ListenAndServe()
}

func loadWorkflow(path string) (workflow.Workflow, error) {
	wf, err := workflow.LoadFile(path)
	if err != nil {
		return workflow.Workflow{}, err
	}
	if err := wf.Validate(); err != nil {
		return workflow.Workflow{}, err
	}
	return wf, nil
}

func defaultRegistry() *modules.Registry {
	cfg, err := config.Load("")
	dataDir := config.DefaultConfig().DataDir
	if err == nil && cfg.DataDir != "" {
		dataDir = cfg.DataDir
	}
	scriptStore := scriptstore.New(filepath.Join(dataDir, "scripts"))

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

func printJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: bops <plan|apply|test|status|serve> -f <workflow.yaml>")
}

func fatal(err error) {
	logging.L().Error("command failed", zap.Error(err))
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
