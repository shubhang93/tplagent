package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/shubhang93/tplagent/internal/config"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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

		err = config.WriteTo(stdout, *numBlocks, *indent)
		if err != nil {
			return err
		}
	case "reload":
		pidFilePath := filepath.Join(pidDir, pidFilename)
		return reload(pidFilePath)
	default:
		return errors.New(usage)
	}
	return nil
}

func reload(pidFilePath string) error {
	contents, err := os.ReadFile(pidFilePath)
	if err != nil {
		return err
	}

	pid, err := strconv.Atoi(string(contents))
	if err != nil {
		return fmt.Errorf("error converting PID:%w", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("proc find err:%w", err)
	}

	err = proc.Signal(syscall.SIGHUP)
	if err != nil {
		return fmt.Errorf("sig send err:%w", err)
	}
	return nil
}
