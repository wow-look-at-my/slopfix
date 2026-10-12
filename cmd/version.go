package cmd

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"
)

// buildInfo is what Go's VCS stamping recorded in this binary.
type buildInfo struct {
	Revision string
	Time     time.Time
	Modified bool
}

// readBuildInfo reads the VCS stamp. A binary built outside a checkout has none.
func readBuildInfo() buildInfo {
	var b buildInfo
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return b
	}
	return parseBuildSettings(bi.Settings)
}

func parseBuildSettings(settings []debug.BuildSetting) buildInfo {
	var b buildInfo
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			b.Revision = s.Value
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
				b.Time = t
			}
		case "vcs.modified":
			b.Modified = s.Value == "true"
		}
	}
	return b
}

// version is v0.0.<commit time in Unix seconds>, the numbering go-toolchain's own version command uses.
func (b buildInfo) version() string {
	if b.Time.IsZero() {
		return "dev"
	}
	return fmt.Sprintf("v0.0.%d", b.Time.Unix())
}

// String is the line both `--version` and `version` print.
func (b buildInfo) String() string {
	if b.Revision == "" {
		return b.version()
	}
	s := fmt.Sprintf("%s (%s, %s)", b.version(), b.Revision, b.Time.UTC().Format(time.RFC3339))
	if b.Modified {
		s += " modified"
	}
	return s
}

func init() {
	info := readBuildInfo()
	rootCmd.Version = info.String()
	rootCmd.SetVersionTemplate("slopfix {{.Version}}\n")
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the build's version, commit and commit time",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "slopfix %s\n", info)
			return err
		},
	})
}
