package cmdexec

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

func Do(ctx context.Context, cmd string, args ...string) error {

	cmdPath, err := exec.LookPath(cmd)
	if errors.Is(err, exec.ErrNotFound) {
		return err
	}

	runErr := exec.CommandContext(ctx, cmdPath, args...).Run()

	var exitErr *exec.ExitError
	if runErr != nil && errors.As(runErr, &exitErr) {
		fmt.Println(string(exitErr.Stderr))
		return fmt.Errorf("command failed with status:%d", exitErr.ExitCode())
	}

	if runErr != nil {
		return fmt.Errorf("command failed with error:%w", runErr)
	}
	return nil
}
