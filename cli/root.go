// Package cli assembles the pixiv command tree from the pixiv
// domain on top of the any-cli/kit framework.
package cli

import (
	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/pixiv-cli/pixiv"
)

// Build metadata, set via -ldflags at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// NewApp assembles the kit application from the pixiv domain. The domain's
// Register installs the client factory and every operation, so the binary and a
// host that blank-imports the pixiv package (ant) share one source of truth.
// kit.Run turns the App into the CLI, plus the serve and mcp surfaces and the
// typed-error-to-exit-code mapping.
func NewApp() *kit.App {
	id := pixiv.Domain{}.Info().Identity
	id.Version = Version

	app := kit.New(id)
	(pixiv.Domain{}).Register(app)
	registerVersion(app)
	return app
}

// Cli is the entry point for the pixiv binary. It stays a call rather than a
// package variable because Version is injected with -ldflags into this package:
// a package-level App would freeze whatever the linker saw at init time.
func Cli() { kit.Main(NewApp()) }
