package cmd

import (
	"github.com/amirdaaee/Glide/cmd/wire"
	"github.com/amirdaaee/Glide/internals/bot"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var botCmd = &cobra.Command{
	Use:   "bot",
	Short: "Start the Telegram bot",
	RunE: func(cmd *cobra.Command, args []string) error {
		ll := log.GetLogger(log.CMD).Named("bot")
		ll.Info("starting")
		p := wire.GetProvider()
		err := p.Invoke(func(b *bot.Bot) error {
			return b.Start(cmd.Context())
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
	rootCmd.AddCommand(botCmd)
}
