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
	cli "github.com/urfave/cli/v3"
)

func runCommand(ctx context.Context, cmd *cli.Command) error {
	cfgName := cmd.String("config")
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

	b, err := bot.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize bot: %w", err)
	}
	defer b.Close()

	return b.Start(ctx)
}

func main() {
	cmd := &cli.Command{
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

	if err := cmd.Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error : %v\n", err)

		var exitCoder cli.ExitCoder
		if errors.As(err, &exitCoder) {
			os.Exit(exitCoder.ExitCode())
		}
	}
}
