package cmd

import (
	"github.com/amirdaaee/Glide/cmd/wire"
	"github.com/amirdaaee/Glide/internals/bot"
	"github.com/spf13/cobra"
)

var botCmd = &cobra.Command{
	Use:   "bot",
	Short: "Start the Telegram bot",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := wire.GetProvider()
		return p.Invoke(func(b *bot.Bot) error {
			return b.Start(cmd.Context())
		})
	},
}

func init() {
	rootCmd.AddCommand(botCmd)
}
