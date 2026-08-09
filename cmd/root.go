/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/amirdaaee/Glide/internals/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "Glide",
	Short: "Glide is a tool for managing your Telegram media library",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		dev, _ := cmd.Flags().GetBool("dev")
		level, _ := cmd.Flags().GetString("log-level")
		log.Setup(dev, level)
		registerGracefulShutdown(cmd)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolP("dev", "d", false, "Run in development mode")
	rootCmd.PersistentFlags().StringP("log-level", "l", "warning", "Log level")
	rootCmd.SilenceUsage = true
}

func registerGracefulShutdown(cmd *cobra.Command) {
	ctx, cancel := context.WithCancel(cmd.Context())
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdown
		zap.L().Warn("Shutting down gracefully...")
		cancel()
	}()
	cmd.SetContext(ctx)
}
