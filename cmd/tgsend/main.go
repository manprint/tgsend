package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/manprint/tgsend/internal/app"
	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/buildinfo"
	"github.com/manprint/tgsend/internal/cli"
	"github.com/manprint/tgsend/internal/config"
	"github.com/manprint/tgsend/internal/message"
	"github.com/manprint/tgsend/internal/telegram"
)

const apiBaseURLEnv = "TGSEND_API_BASE_URL"

type telegramSender struct {
	baseURL string
	doer    telegram.Doer
	sleeper telegram.Sleeper
}

func (sender telegramSender) Send(ctx context.Context, cfg config.Config, chunk message.Chunk) (int64, error) {
	client, err := telegram.NewClient(telegram.Options{
		Token:   cfg.Token,
		BaseURL: sender.baseURL,
		Doer:    sender.doer,
		Sleeper: sender.sleeper,
	})
	if err != nil {
		return 0, err
	}
	return client.Send(ctx, cfg.ChatID, chunk)
}

func main() {
	baseURL, preflightErr := configuredAPIBaseURL()
	service := app.NewService(os.Stdin, app.IsTerminal(os.Stdin))
	service.Sender = telegramSender{
		baseURL: baseURL,
		doer:    &http.Client{Timeout: 10 * time.Second},
		sleeper: telegram.NewProductionSleeper(),
	}
	service.PreflightError = preflightErr

	deps := cli.Dependencies{
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		BuildInfo: buildinfo.Current(),
		App:       service,
	}
	os.Exit(cli.Execute(context.Background(), deps, os.Args[1:]))
}

func configuredAPIBaseURL() (string, error) {
	value, ok := os.LookupEnv(apiBaseURLEnv)
	if !ok || value == "" {
		return "", nil
	}
	return resolveAPIBaseURL(value, buildinfo.TestEndpointEnabled)
}

func resolveAPIBaseURL(value, endpointEnabled string) (string, error) {
	if endpointEnabled != "true" {
		return "", invalidAPIBaseURL(errors.New("test endpoint is disabled"))
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", invalidAPIBaseURL(err)
	}
	host := parsed.Hostname()
	if host != "localhost" && !net.ParseIP(host).IsLoopback() {
		return "", invalidAPIBaseURL(errors.New("test endpoint is not loopback"))
	}
	return value, nil
}

func invalidAPIBaseURL(cause error) error {
	return apperr.New(apperr.KindUsage, apperr.CodeInvalidArguments, "TGSEND_API_BASE_URL is restricted to loopback HTTP in test builds", cause)
}

var _ app.Sender = telegramSender{}
