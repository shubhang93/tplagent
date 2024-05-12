package main

import (
	"context"
	"fmt"
	"github.com/shubhang93/tplagent/internal/config"
	"log/slog"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func Test_reloadProcs(t *testing.T) {

	type withoutHTTPServerTest struct {
		WantError           error
		AtleastReloadNTimes int32
		AgentErr            func(num int32) error
	}

	tests := map[string]withoutHTTPServerTest{
		"without error": {
			AtleastReloadNTimes: 8,
			AgentErr: func(_ int32) error {
				return nil
			},
		},
		"with error": {
			AtleastReloadNTimes: 8,
			AgentErr: func(num int32) error {
				return fmt.Errorf("agent error %d", num)
			},
		},
	}

	t.Run("without HTTP server", func(t *testing.T) {
		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				tmp := t.TempDir()
				cfgFile := tmp + "/config.json"
				f, err := os.Create(cfgFile)
				if err != nil {
					t.Error(err)
					return
				}
				err = config.WriteTo(f, 1, 1)
				if err != nil {
					t.Error(err)
					return
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				agentReloadCount := atomic.Int32{}
				ps := procStarters{
					listener: func(ctx context.Context, conf config.TPLAgent, reload bool) error {
						return nil
					},
					agent: func(ctx context.Context, conf config.TPLAgent, reload bool) error {
						t.Log("reloading")
						agentReloadCount.Add(1)
						return test.AgentErr(agentReloadCount.Load())
					},
				}

				process, err := os.FindProcess(os.Getpid())
				if err != nil {
					t.Error(err)
					return
				}

				done := make(chan struct{})
				errCh := make(chan error, 1)
				go func() {
					errCh <- reloadProcs(ctx, cfgFile, ps)
					close(done)
				}()

				run := true
				for run {
					select {
					case <-ctx.Done():
						run = false
					default:
						time.Sleep(500 * time.Millisecond)
						err := process.Signal(syscall.SIGHUP)
						if err != nil {
							t.Error(err)
							run = false
						}
					}
				}

				<-done

				if agentReloadCount.Load() < test.AtleastReloadNTimes {
					t.Errorf("got %d expected atleast %d", agentReloadCount.Load(), test.AtleastReloadNTimes)
				}

				gotErr := <-errCh
				t.Logf("agent error %v", gotErr)
			})
		}

	})
}

var makeConfig = func(suffix string, tmpDir string) config.TPLAgent {
	return config.TPLAgent{
		Agent: config.Agent{
			LogLevel: slog.LevelInfo,
			LogFmt:   "text",
		},
		TemplateSpecs: map[string]*config.TemplateSpec{
			"templ1": {
				Raw:         fmt.Sprintf("{{.Name_%s}}", suffix),
				Destination: tmpDir + "/dest.render" + suffix,
			},
			"templ2": {
				Raw:         "{{.ID}}",
				Destination: tmpDir + "/dest2.render" + suffix,
			},
		},
	}
}
