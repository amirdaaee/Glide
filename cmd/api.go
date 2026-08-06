/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/amirdaaee/Glide/cmd/wire"
	"github.com/amirdaaee/Glide/internals/api"
	"github.com/spf13/cobra"
)

// apiCmd represents the api command
var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Start the API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := wire.GetProvider()
		return p.Invoke(func(apiServer *api.ApiServer) error {
			return apiServer.Start(cmd.Context())
		})
	},
}

func init() {
	rootCmd.AddCommand(apiCmd)
}
