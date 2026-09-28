package cmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/langserver"
)

// lspMaxPerFile caps the diagnostics a single file publishes.
var lspMaxPerFile int

func init() {
	command := &cobra.Command{
		Use:   "lsp",
		Short: "Serve the findings to an editor as a language server on stdio",
		Long: "lsp speaks the Language Server Protocol on stdin and stdout. Each open\n" +
			"file that a build reads is published with the findings report gives for\n" +
			"it: a workflow, an action manifest, a document or a source file inside a\n" +
			"work tree. A file outside every work tree, or under ~/.claude, is left\n" +
			"alone, because no build reads it.",
		Args: cobra.NoArgs,
		RunE: runLSP,
	}
	command.Flags().IntVar(&lspMaxPerFile, "max-per-file", 10,
		"publish at most this many diagnostics per file, 0 for no cap. The last one sent counts the rest")
	rootCmd.AddCommand(command)
}

func runLSP(cmd *cobra.Command, _ []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	langserver.New(lspMaxPerFile, home).Serve(cmd.Context(), stdio{Reader: cmd.InOrStdin(), Writer: cmd.OutOrStdout()})
	return nil
}

// stdio is the process's own streams as the connection a server holds.
type stdio struct {
	io.Reader
	io.Writer
}

func (stdio) Close() error { return nil }
