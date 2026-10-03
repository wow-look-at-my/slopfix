package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/pluginmodule"
)

// hook.go sorts before this file, so its init has set hookCmd.
func init() {
	hookCmd.AddCommand(&cobra.Command{
		Use:   "module DIR",
		Short: "Write the hooks module of cc-marketplace's plugins/slopfix into DIR",
		Long: "module writes register.ts and register.test.ts into DIR, which it creates.\n" +
			"The plugin's hooks.json names register.ts. The module answers a Bash file\n" +
			"read with Read calls, and asks `check read-plan` which calls those are. It\n" +
			"ships inside this binary so the module and the command it calls match.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := pluginmodule.Write(args[0]); err != nil {
				return fmt.Errorf("hook module: %w", err)
			}
			for _, name := range pluginmodule.Names {
				fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(args[0], name))
			}
			return nil
		},
	})
}
