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
		env         map[string]string
		beforeFunc  func()
		afterFunc   func(t *testing.T)
		cmd         string
		expectError bool
	}
	temp := t.TempDir()
	scriptName := "script.sh"
	execTests := []execTest{{
		name:        "bin exists",
		cmd:         `echo`,
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
	}, {
		name:        "command exits with a non zero exit code",
		cmd:         "bash",
		args:        []string{"-c", "exit 1"},
		expectError: true,
	},
		{
			name:       "sets custom env for command",
			beforeFunc: nil,
			cmd:        "bash",
			args: []string{
				"-c",
				fmt.Sprintf(`%s "%s" > %s`, "echo -n", "hello $USERNAME", temp+"/test.out"),
			},
			env:         map[string]string{"USERNAME": "Foo"},
			expectError: false,
			afterFunc: func(t *testing.T) {
				bs, err := os.ReadFile(temp + "/test.out")
				if err != nil {
					t.Error(err)
					return
				}
				expected := `hello Foo`
				got := string(bs)
				if expected != got {
					t.Errorf("-(%s) +(%s)", expected, got)
				}
			},
		}}

	for _, et := range execTests {
		t.Run(et.name, func(t *testing.T) {
			if et.beforeFunc != nil {
				et.beforeFunc()
			}

			if et.afterFunc != nil {
				defer et.afterFunc(t)
			}

			defaultExecer := Default{
				Cmd:     et.cmd,
				Args:    et.args,
				Env:     et.env,
				Timeout: 10 * time.Second,
			}
			err := defaultExecer.ExecContext(context.Background())
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
