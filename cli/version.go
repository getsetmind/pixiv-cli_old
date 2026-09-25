package cli

import (
	"context"
	"runtime"

	"github.com/tamnd/any-cli/kit"
)

// versionInfo is the record the version op emits. The fields come straight from
// the ldflags variables in root.go rather than kit.NewVersion, which carries a
// version and nothing else: the commit and build date would be lost.
type versionInfo struct {
	Version  string `json:"version"  kit:"id" table:"version"`
	Commit   string `json:"commit"             table:"commit"`
	Built    string `json:"built"              table:"built"`
	Platform string `json:"platform"           table:"platform"`
	Go       string `json:"go"                 table:"go"`
}

type versionInput struct {
	Short bool `kit:"flag" help:"print just the version number"`
}

// registerVersion installs the version op. It is a normal operation rather than
// a hand-written command so that every output flag the rest of the CLI honours
// -- -o, --fields, --template, --db -- applies here too; a command that wrote to
// os.Stdout itself silently ignored them.
func registerVersion(app *kit.App) {
	kit.Handle(app, kit.OpMeta{
		Name:    "version",
		Summary: "Print version information",
	}, runVersion)
}

func runVersion(_ context.Context, in versionInput, emit func(*versionInfo) error) error {
	v := Version
	if !in.Short {
		v = "pixiv " + Version
	}
	return emit(&versionInfo{
		Version:  v,
		Commit:   Commit,
		Built:    Date,
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
		Go:       runtime.Version(),
	})
}
