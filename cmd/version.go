package cmd

import (
	"fmt"
	"runtime"

	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// versionInfo is the JSON shape emitted by `lincli version --json`.
type versionInfo struct {
	Version   string `json:"version"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// buildVersionInfo gathers the build version (stamped via -ldflags, or "dev"
// for a local build) together with the Go toolchain and target platform. It is
// a separate function so tests can assert the shape without a real build stamp.
func buildVersionInfo() versionInfo {
	return versionInfo{
		Version:   version,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// versionCmd prints the build version. cobra's --version flag (set via
// rootCmd.Version) already covers the flag form; this adds the `lincli version`
// subcommand and honors the shared output modes.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the lincli version",
	Long: `Show the lincli build version, plus the Go toolchain and platform it was
built for. The version is the git tag or short commit the binary was built from,
or "dev" for an un-stamped local build.

Examples:
  lincli version
  lincli version --plaintext   # just the version string, for scripts
  lincli version --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		info := buildVersionInfo()
		switch {
		case jsonOut:
			output.JSON(info)
		case plaintext:
			fmt.Println(info.Version)
		default:
			fmt.Printf("lincli version %s\n", info.Version)
			fmt.Printf("  go:       %s\n", info.GoVersion)
			fmt.Printf("  platform: %s\n", info.Platform)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
