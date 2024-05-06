package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/shubhang93/tplagent/internal/agent"
	"io"
	"runtime"
	"strings"
)

const usage = `usage:

  tplagent start -config=/path/to/config.json
    -config: specifies the path to read the config file from (default /etc/tplagent/config.json)

  tplagent genconf -n 1 -indent 4 > path/to/config.json
    -n:      number of template blocks to generate (default 1)
    -indent: indentation space in the generated config (default 2)
	
  tplagent version
`

const defaultConfigPath = "/etc/tplagent/config.json"

func startCLI(ctx context.Context, stdout io.Writer, args ...string) error {
	if len(args) < 1 {
		return errors.New(usage)
	}

	startCmd := flag.NewFlagSet("start", flag.ExitOnError)
	configPath := startCmd.String("config", defaultConfigPath, "-config /path/to/config.json")

	genConfCmd := flag.NewFlagSet("genconf", flag.ExitOnError)
	numBlocks := genConfCmd.Int("n", 1, "-n 2")
	indent := genConfCmd.Int("indent", 2, "-indent 2")

	cmd := args[0]
	args = args[1:]
	switch cmd {
	case "version":
		versionInfoTemplate := `Agent Version: %s
Go Runtime: %s`
		GoVer := strings.TrimLeft(runtime.Version(), "go")
		_, _ = fmt.Fprintf(stdout, versionInfoTemplate, version, GoVer)
	case "start":
		err := startCmd.Parse(args)
		if err != nil {
			return err
		}
		return startAgent(ctx, *configPath)
	case "genconf":
		err := genConfCmd.Parse(args)
		if err != nil {
			return err
		}

		if *numBlocks < 1 {
			*numBlocks = 1
		}

		if *indent < 1 {
			*indent = 2
		}

		err = agent.WriteConfig(stdout, *numBlocks, *indent)
		if err != nil {
			return err
		}
	default:
		return errors.New(usage)
	}
	return nil

}
