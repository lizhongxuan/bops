package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"bops/internal/agent"
)

func main() {
	fs := flag.NewFlagSet("bops-agent", flag.ExitOnError)
	id := fs.String("id", "agent-local", "agent id")
	interval := fs.Duration("heartbeat", 10*time.Second, "heartbeat interval")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ag := agent.New(*id, []string{"cmd.run", "shell.run", "script.shell", "script.python", "env.set", "template.render", "wait.event"})
	ag.Start()
	printInfo(ag.Info())

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for range ticker.C {
		ag.Heartbeat()
		printInfo(ag.Info())
	}
}

func printInfo(info agent.Info) {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(info)
}
