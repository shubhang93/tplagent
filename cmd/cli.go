package main

import (
	"context"
	"errors"
	"flag"
	"github.com/shubhang93/tplagent/internal/agent"
	"io"
)

const usage = `usage: 
  tplagent start -config=/path/to/config
  tplagent generate > path/to/config.json`
const defaultConfigPath = "/etc/tplagent/config.json"

func startCLI(ctx context.Context, stdout io.Writer, args ...string) error {
	if len(args) < 1 {
		return errors.New(usage)
	}

	startCmd := flag.NewFlagSet("start", flag.ExitOnError)
	configPath := startCmd.String("config", defaultConfigPath, "-config /path/to/config.json")

	cmd := args[0]
	args = args[1:]
	switch cmd {
	case "start":
		err := startCmd.Parse(args)
		if err != nil {
			return err
		}
		return startAgent(ctx, *configPath)
	case "generate":
		err := agent.GenerateConfig(stdout)
		if err != nil {
			return err
		}
	default:
		return errors.New(usage)
	}
	return nil

}
