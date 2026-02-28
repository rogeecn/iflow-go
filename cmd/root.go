package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "iflow-go",
	Short:         "iFlow API 代理服务",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}
