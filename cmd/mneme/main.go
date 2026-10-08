package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/clobrano/mneme/internal/cli"
	"github.com/clobrano/mneme/internal/config"
)

func main() {
	var configPath string
	var jsonOutput bool
	var noTUI bool

	flag.StringVar(&configPath, "config", "", "path to config file (default: XDG config path)")
	flag.BoolVar(&jsonOutput, "json", false, "output JSON instead of human-readable text")
	flag.BoolVar(&noTUI, "no-tui", false, "disable TUI, use plain stdout")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mneme [options] <command> [args]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  sync       [--watch]           Reindex notes and detect orphans\n")
		fmt.Fprintf(os.Stderr, "  review     [--tag T] [--limit N] Review due cards\n")
		fmt.Fprintf(os.Stderr, "  orphans                        List orphaned cards\n")
		fmt.Fprintf(os.Stderr, "  resolve    [--undo]            Manually resolve orphans\n")
		fmt.Fprintf(os.Stderr, "  stats                          Show deck statistics\n")
		fmt.Fprintf(os.Stderr, "  generate   [--json]            Generate cards from notes (no DB)\n")
		fmt.Fprintf(os.Stderr, "\nGlobal options:\n")
		flag.PrintDefaults()
	}

	// Parse only up to the first non-flag argument (the subcommand)
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Resolve config path
	if configPath == "" {
		configPath = config.XDGConfigPath()
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}
	cfg.NotesDir = config.ExpandPath(cfg.NotesDir)
	if cfg.Database != "" {
		cfg.Database = config.ExpandPath(cfg.Database)
	} else {
		cfg.Database = config.XDGStatePath()
	}

	subcmd := flag.Arg(0)
	args := flag.Args()[1:]

	opts := cli.Options{
		Config:     cfg,
		JSONOutput: jsonOutput,
		NoTUI:      noTUI,
	}

	switch subcmd {
	case "sync":
		os.Exit(cli.RunSync(opts, args))
	case "review":
		os.Exit(cli.RunReview(opts, args))
	case "orphans":
		os.Exit(cli.RunOrphans(opts, args))
	case "resolve":
		os.Exit(cli.RunResolve(opts, args))
	case "stats":
		os.Exit(cli.RunStats(opts, args))
	case "generate":
		os.Exit(cli.RunGenerate(opts, args))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", subcmd)
		flag.Usage()
		os.Exit(1)
	}
}
