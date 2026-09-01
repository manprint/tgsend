// Package cli wires Cobra to the process-level dependencies and output rules.
package cli

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/manprint/tgsend/internal/app"
	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/buildinfo"
	"github.com/manprint/tgsend/internal/presenter"
	"github.com/spf13/cobra"
)

// Dependencies are the streams and immutable metadata needed by the root CLI.
type Dependencies struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	BuildInfo buildinfo.Info
	App       Runner
}

// Runner is the application boundary invoked after Cobra has parsed flags.
type Runner interface {
	Run(context.Context, app.Options) (presenter.SendResult, error)
}

// NewRoot creates a root command without process exits or global side effects.
func NewRoot(deps Dependencies) *cobra.Command {
	root := &cobra.Command{
		Use:           "tgsend",
		Short:         "Send a message to Telegram",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          noPositionalArgs,
	}
	root.SetIn(deps.Stdin)
	root.SetOut(deps.Stdout)
	root.SetErr(deps.Stderr)

	var version bool
	var message string
	var configPath string
	var title string
	var messageType string
	var monospace bool
	var silent bool
	var dryRun bool
	var maxInputBytes int64
	root.Flags().BoolVar(&version, "version", false, "print version information as JSON")
	root.Flags().StringVarP(&message, "message", "m", "", "message text (mutually exclusive with stdin)")
	root.Flags().StringVarP(&configPath, "config", "c", "", "configuration file path")
	root.Flags().StringVar(&title, "title", "", "optional bold title")
	root.Flags().StringVar(&messageType, "type", "", "optional type: INFO, WARNING, ERROR, or CRITICAL")
	root.Flags().BoolVar(&monospace, "monospace", false, "format each body chunk as preformatted text")
	root.Flags().BoolVar(&silent, "silent", false, "disable Telegram notifications")
	root.Flags().BoolVar(&dryRun, "dry-run", false, "validate and preview without credentials or network")
	root.Flags().Int64Var(&maxInputBytes, "max-input-bytes", 1<<20, "maximum input size in bytes")
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		if version {
			return presenter.WriteVersion(deps.Stdout, deps.BuildInfo)
		}
		if deps.App == nil {
			return apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, "sending is not available in this build phase", nil)
		}
		result, err := deps.App.Run(cmd.Context(), app.Options{
			Message:        message,
			MessageSet:     cmd.Flags().Changed("message"),
			ConfigPath:     configPath,
			ConfigExplicit: cmd.Flags().Changed("config"),
			Title:          title,
			Type:           messageType,
			Monospace:      monospace,
			Silent:         silent,
			DryRun:         dryRun,
			MaxInputBytes:  maxInputBytes,
		})
		if err != nil {
			return err
		}
		return presenter.WriteSend(deps.Stdout, result)
	}
	return root
}

func noPositionalArgs(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("positional arguments are not allowed")
	}
	return nil
}

// Execute runs the CLI and returns its stable process exit code.
func Execute(ctx context.Context, deps Dependencies, args []string) int {
	if ctx == nil {
		ctx = context.Background()
	}
	root := NewRoot(deps)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		appErr := classifyCobraError(err)
		_ = presenter.WriteError(deps.Stderr, "send", presenter.ErrorBodyFrom(appErr))
		return apperr.ExitCode(appErr)
	}
	return 0
}

func classifyCobraError(err error) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr != nil {
		return appErr
	}

	if strings.Contains(err.Error(), "flag") {
		return apperr.New(apperr.KindUsage, apperr.CodeInvalidFlag, "invalid command-line flag", err)
	}
	return apperr.New(apperr.KindUsage, apperr.CodeInvalidArguments, "positional arguments are not allowed", err)
}
