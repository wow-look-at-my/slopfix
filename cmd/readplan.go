package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/bashclean"
)

// readPlanInput is what the plugin's tool.call module sends for one Bash call.
type readPlanInput struct {
	Command string `json:"command"`
	Cwd     string `json:"cwd"`
}

// check.go sorts before this file, so its init has set checkCmd.
func init() {
	checkCmd.AddCommand(&cobra.Command{
		Use:   "read-plan",
		Short: "Map a Bash file read onto Read tool calls",
		Long: "read-plan reads {\"command\", \"cwd\"} as JSON on stdin. A plain cat, head,\n" +
			"tail, sed -n or line-number awk read prints {\"reads\": [...], \"note\": \"...\"}: one Read\n" +
			"input for each file, and the note the model gets after the result. Any\n" +
			"other command prints {\"reads\": null}, and the caller runs it as written.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var in readPlanInput
			if err := json.NewDecoder(cmd.InOrStdin()).Decode(&in); err != nil {
				return fmt.Errorf("read-plan: stdin is not a {command, cwd} object: %w", err)
			}
			plan := bashclean.PlanRead(in.Command, in.Cwd)
			if plan == nil {
				plan = &bashclean.ReadPlan{}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(plan)
		},
	})
}
