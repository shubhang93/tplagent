package cmdexec

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestDo(t *testing.T) {

	type execTest struct {
		name        string
		args        []string
		beforeFunc  func()
		cmd         string
		timeout     time.Duration
		expectError bool
	}
	temp := t.TempDir()
	scriptName := "script.sh"
	execTests := []execTest{{
		name:        "bin exists",
		cmd:         `echo "hello world"`,
		args:        []string{"hello world"},
		expectError: false,
	}, {
		name:        "bin does not exist",
		cmd:         "foobarfoo",
		args:        []string{"123"},
		expectError: true,
	}, {
		name: "run command from script",
		beforeFunc: func() {
			scriptPath := fmt.Sprintf("%s/%s", temp, scriptName)
			fi, err := os.OpenFile(scriptPath, os.O_CREATE|os.O_RDWR, 0755)
			if err != nil {
				t.Errorf("failed to create script:%v", err)
				return
			}
			_, err = fi.WriteString(`#!/bin/bash
echo "hello world from script"
`)
			if err != nil {
				t.Errorf("error writing to script:%v", err)
				return
			}
		},
		cmd:  "bash",
		args: []string{"-c", fmt.Sprintf("%s/%s", temp, scriptName)},
	}}

	for _, et := range execTests {
		t.Run(et.name, func(t *testing.T) {
			if et.beforeFunc != nil {
				et.beforeFunc()
			}
			var ctx = context.Background()
			var cancel context.CancelFunc = func() {}
			if et.timeout > 0 {
				ctx, cancel = context.WithTimeout(context.Background(), et.timeout)
			}
			defer cancel()
			defaultExecer := Default{
				Cmd:  et.cmd,
				Args: et.args,
			}
			err := defaultExecer.ExecContext(ctx)
			if et.expectError && err == nil {
				t.Errorf("expected error got nil")
				return
			}
			if et.expectError {
				t.Logf("error:%v", err)
			}
		})
	}

}
