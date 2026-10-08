package cli

import "github.com/clobrano/mneme/internal/config"

// Options carries shared configuration for all CLI commands.
type Options struct {
	Config     *config.Config
	JSONOutput bool
	NoTUI      bool
}
