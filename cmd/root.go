package cmd

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/k0kubun/pp/v3"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"

	"github.com/ikorchynskyi/instance-stack-curator/internal/types"
	"github.com/ikorchynskyi/instance-stack-curator/internal/validator"
)

const ()

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "instance-stack-curator",
	Short: "EC2 instance stack curator",
	Long: `A CLI application to curate an ASG based stacks of EC2 instances.

It allows to execute startup and shutdown of groups of EC2 instances in a predicted sequentional manner.
	`,
}

var debug bool
var stack types.Stack
var stackFile string

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// DisableDefaultCmd prevents Cobra from creating a default 'completion' command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// SilenceUsage is an option to silence usage when an error occurs.
	rootCmd.SilenceUsage = true

	// Persistent flags which will be global for the application.
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Turn on debug logging")
	rootCmd.PersistentFlags().StringVar(&stackFile, "stack", "", "Path to a stack spec")
	rootCmd.MarkPersistentFlagRequired("stack")

	pp.PrintMapTypes = false
	pp.Default.SetExportedOnly(true)
	pp.Default.SetColoringEnabled(term.IsTerminal(int(os.Stdout.Fd())))
}

func initStack() error {
	stackYaml, err := ioutil.ReadFile(stackFile)
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal([]byte(stackYaml), &stack); err != nil {
		return err
	}

	if err = validator.ValidateStack(&stack); err != nil {
		return err
	}

	pp.Printf("Instance stack: %v\n", stack)
	return nil
}

func initAWS() (aws.Config, error) {
	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	var clientLogMode aws.ClientLogMode
	if debug {
		clientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
	} else {
		clientLogMode = 0
	}

	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithClientLogMode(clientLogMode),
	)
	return cfg, err
}
