package cmdexec

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

type Default struct {
	Args    []string
	Cmd     string
	Env     map[string]string
	Timeout time.Duration
}

func (d *Default) ExecContext(ctx context.Context) error {

	ctxWithTimeout, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()

	cmdPath, err := exec.LookPath(d.Cmd)
	if errors.Is(err, exec.ErrNotFound) {
		return err
	}

	cmd := exec.CommandContext(ctxWithTimeout, cmdPath, d.Args...)
	setEnv(cmd, d.Env)

	runErr := cmd.Run()

	var exitErr *exec.ExitError
	if runErr != nil && errors.As(runErr, &exitErr) {
		return fmt.Errorf("command failed with status:%d", exitErr.ExitCode())
	}

	if runErr != nil {
		return fmt.Errorf("command failed with error:%w", runErr)
	}
	return nil
}

func setEnv(c *exec.Cmd, env map[string]string) {
	for k, v := range env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
	}
}
