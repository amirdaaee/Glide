package cmd

import (
	"github.com/amirdaaee/Glide/cmd/wire"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/orchestrator"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var orchestratorCmd = &cobra.Command{
	Use:   "orchestrator",
	Short: "Start the media pipeline orchestrator",
	RunE: func(cmd *cobra.Command, args []string) error {
		ll := log.GetLogger(log.CMD).Named("orchestrator")
		ll.Info("starting")
		p := wire.GetProvider()
		err := p.Invoke(func(o *orchestrator.Orchestrator) error {
			return o.Start(cmd.Context())
		})
		if err != nil && cmd.Context().Err() == nil {
			ll.Error("exited with error", zap.Error(err))
			return err
		}
		ll.Info("stopped")
		return err
	},
}

func init() {
	rootCmd.AddCommand(orchestratorCmd)
}
