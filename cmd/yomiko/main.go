package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"github.com/kechako/yomiko/bot"
	"github.com/urfave/cli/v2"
)

func runCommand(c *cli.Context) error {
	cfgName := c.String("config")
	if cfgName == "" {
		return errors.New("config file is not specified")
	}

	cfg, err := bot.ReadConfigFile(cfgName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return errors.New("config file is not found")
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}

	ctx := c.Context

	b, err := bot.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize bot: %w", err)
	}
	defer b.Close()

	return b.Start(ctx)
}

func main() {
	app := &cli.App{
		Name: "yomiko",
		Commands: []*cli.Command{
			{
				Name: "run",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Value:   "config.toml",
					},
				},
				Action: runCommand,
			},
		},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	if err := app.RunContext(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error : %v\n", err)

		var exitCoder cli.ExitCoder
		if errors.As(err, &exitCoder) {
			os.Exit(exitCoder.ExitCode())
		}
	}
}
