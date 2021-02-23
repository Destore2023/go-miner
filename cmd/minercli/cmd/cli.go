package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   filepath.Base(os.Args[0]),
	Short: "Command line client for sukhavati miner",
}
