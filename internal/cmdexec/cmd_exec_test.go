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
		expectError: false,
	}, {
		name:        "bin does not exist",
		cmd:         "foobarfoo 123",
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
		cmd: fmt.Sprintf("bash -c %s/%s", temp, scriptName),
	}, {
		name:        "malformed command",
		cmd:         "bash          -c",
		timeout:     0,
		expectError: true,
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
			err := Do(ctx, et.cmd)
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
