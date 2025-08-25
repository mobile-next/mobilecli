package cli

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/mobile-next/mobilecli/utils"
	"github.com/spf13/cobra"
)

const version = "dev"

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "mobilecli",
	Short: "A cross-platform iOS/Android device automation tool",
	Long:  `A universal tool for managing iOS and Android devices`,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func initConfig() {
	utils.SetVerbose(verbose)
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}

// Execute runs the root command
func Execute() error {
	// enable microseconds in logs
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	return rootCmd.Execute()
}

// printJson is a helper function to print JSON responses
func printJson(data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(jsonData))
}
