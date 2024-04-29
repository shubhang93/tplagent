package cmdexec

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func Do(ctx context.Context, cmd string) error {

	cmdComponents := strings.Fields(cmd)
	if len(cmdComponents) < 1 {
		return errors.New("malformed command")
	}
	binName := cmdComponents[0]
	cmdComponents = cmdComponents[1:]
	cmdPath, err := exec.LookPath(binName)
	if errors.Is(err, exec.ErrNotFound) {
		return err
	}

	argsLen := len(cmdComponents)
	var args []string
	if argsLen > 0 {
		args = cmdComponents
	}

	command := exec.CommandContext(ctx, cmdPath, args...)
	runErr := command.Run()
	var exitErr *exec.ExitError
	if runErr != nil && errors.As(runErr, &exitErr) {
		return fmt.Errorf("command failed with status:%d", exitErr.ExitCode())
	}
	if runErr != nil {
		return fmt.Errorf("command failed with error:%w", runErr)
	}
	return nil
}
