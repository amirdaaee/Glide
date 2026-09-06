package cmd

import (
	"github.com/amirdaaee/Glide/cmd/wire"
	"github.com/amirdaaee/Glide/internals/orchestrator"
	"github.com/spf13/cobra"
)

var orchestratorCmd = &cobra.Command{
	Use:   "orchestrator",
	Short: "Start the media pipeline orchestrator",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := wire.GetProvider()
		return p.Invoke(func(o *orchestrator.Orchestrator) error {
			return o.Start(cmd.Context())
		})
	},
}

func init() {
	rootCmd.AddCommand(orchestratorCmd)
}
