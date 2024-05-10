package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/shubhang93/tplagent/internal/config"
	"github.com/shubhang93/tplagent/internal/fatal"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

type mockAgent func(ctx context.Context, config config.TPLAgent) error

func (m mockAgent) Start(ctx context.Context, config config.TPLAgent) error {
	return m(ctx, config)
}

func Test_spawnAndReload(t *testing.T) {

	tmpDir := t.TempDir()

	t.Run("SIGHUP", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
		defer cancel()

		oldConfig := makeConfig("old", tmpDir)
		cfgFileLocation := tmpDir + "/agent-config.json"
		f, err := os.OpenFile(cfgFileLocation, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
		if err != nil {
			t.Errorf("error creating config file:%v", err)
			return
		}

		err = json.NewEncoder(f).Encode(&oldConfig)
		if err != nil {
			t.Errorf("error writing old config:%v", err)
			return
		}
		_ = f.Close()

		p, err := os.FindProcess(os.Getpid())
		if err != nil {
			t.Errorf("failed to get process:%v", err)
			return
		}

		reloadTimes := 5

		var newConfigs []config.TPLAgent
		var expectedContextCauses []string
		for i := range reloadTimes {
			cfgSuffix := fmt.Sprintf("new%d", i)
			newConfig := makeConfig(cfgSuffix, tmpDir)
			newConfigs = append(newConfigs, newConfig)
			expectedContextCauses = append(expectedContextCauses, sighupReceived.Error())
		}

		wait := make(chan struct{})
		go func() {
			defer close(wait)
			time.Sleep(100 * time.Millisecond)
			for i := range reloadTimes {
				cfgFileLocation := tmpDir + "/agent-config.json"
				f, err := os.OpenFile(cfgFileLocation, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
				if err != nil {
					t.Errorf("error creating config file:%v", err)
					return
				}
				newConfig := newConfigs[i]
				if err := json.NewEncoder(f).Encode(&newConfig); err != nil {
					t.Errorf("error writing config:%d:%v", i, err)
					return
				}
				_ = f.Close()
				_ = p.Signal(syscall.SIGHUP)
				time.Sleep(100 * time.Millisecond)
			}
		}()

		var gotConfigs []config.TPLAgent
		var gotContextCauses []string

		ma := mockAgent(func(ctx context.Context, config config.TPLAgent) error {

			select {
			case <-ctx.Done():
				gotConfigs = append(gotConfigs, config)
				gotContextCauses = append(gotContextCauses, context.Cause(ctx).Error())
			}

			return nil
		})

		processMaker := func(l *slog.Logger) agentProcess {
			return ma
		}

		err = spawnAndReload(ctx, processMaker, cfgFileLocation)
		if err != nil {
			t.Error(err)
		}

		expectedConfigs := append([]config.TPLAgent{oldConfig}, newConfigs...)

		if diff := cmp.Diff(expectedConfigs, gotConfigs); diff != "" {
			t.Errorf("(--Want ++Got)\n%s", diff)
			return
		}

		expectedContextCauses = append(expectedContextCauses, context.DeadlineExceeded.Error())
		if diff := cmp.Diff(expectedContextCauses, gotContextCauses); diff != "" {
			t.Errorf("(--Want ++Got)\n%s", diff)
		}

		<-wait
	})

	t.Run("SIGHUP with fatal error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
		defer cancel()

		oldConfig := makeConfig("old", tmpDir)

		cfgFileLocation := tmpDir + "/agent-config.json"
		f, err := os.OpenFile(cfgFileLocation, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
		if err != nil {
			t.Errorf("error creating config file:%v", err)
			return
		}

		err = json.NewEncoder(f).Encode(&oldConfig)
		if err != nil {
			t.Errorf("error writing old config:%v", err)
			return
		}
		_ = f.Close()

		p, err := os.FindProcess(os.Getpid())
		if err != nil {
			t.Errorf("failed to get process:%v", err)
			return
		}

		reloadTimes := 5

		var newConfigs []config.TPLAgent
		for i := range reloadTimes {
			cfgSuffix := fmt.Sprintf("new%d", i)
			newConfig := makeConfig(cfgSuffix, tmpDir)
			newConfigs = append(newConfigs, newConfig)
		}

		wait := make(chan struct{})
		go func() {
			defer close(wait)
			time.Sleep(100 * time.Millisecond)
			for i := range reloadTimes {
				cfgFileLocation := tmpDir + "/agent-config.json"
				f, err := os.OpenFile(cfgFileLocation, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
				if err != nil {
					t.Errorf("error creating config file:%v", err)
					return
				}
				newConfig := newConfigs[i]
				if err := json.NewEncoder(f).Encode(&newConfig); err != nil {
					t.Errorf("error writing config:%d:%v", i, err)
					return
				}
				_ = f.Close()
				_ = p.Signal(syscall.SIGHUP)
				time.Sleep(100 * time.Millisecond)
			}
		}()

		var gotConfigs []config.TPLAgent
		var gotContextCauses []string

		ma := mockAgent(func(ctx context.Context, config config.TPLAgent) error {

			select {
			case <-ctx.Done():
				gotConfigs = append(gotConfigs, config)
				gotContextCauses = append(gotContextCauses, context.Cause(ctx).Error())
			}

			return fatal.NewError(errors.New("fatal error"))
		})

		processMaker := func(l *slog.Logger) agentProcess {
			return ma
		}

		err = spawnAndReload(ctx, processMaker, cfgFileLocation)
		if err == nil {
			t.Errorf("expected fatal error got %v", err)
		}

		expectedConfigs := append([]config.TPLAgent{oldConfig})

		if diff := cmp.Diff(expectedConfigs, gotConfigs); diff != "" {
			t.Errorf("(--Want ++Got)\n%s", diff)
			return
		}

		expectedContextCauses := []string{sighupReceived.Error()}
		if diff := cmp.Diff(expectedContextCauses, gotContextCauses); diff != "" {
			t.Errorf("(--Want ++Got)\n%s", diff)
		}
		<-wait
	})

	t.Run("SIGINT,SIGTERM", func(t *testing.T) {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		cfgFileLocation := tmpDir + "/agent-config.json"
		f, err := os.OpenFile(cfgFileLocation, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
		if err != nil {
			t.Errorf("error creating config file:%v", err)
			return
		}

		cfg := makeConfig("test", tmpDir)
		err = json.NewEncoder(f).Encode(cfg)
		if err != nil {
			t.Errorf("error writing old config:%v", err)
			return
		}
		_ = f.Close()

		ma := mockAgent(func(ctx context.Context, cfg config.TPLAgent) error {

			select {
			case <-ctx.Done():
				return ctx.Err()
			}

		})

		p, _ := os.FindProcess(os.Getpid())
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = p.Signal(syscall.SIGINT)
		}()

		pm := func(l *slog.Logger) agentProcess {
			return ma
		}
		_ = spawnAndReload(ctx, pm, cfgFileLocation)
	})

	t.Run("agent returns a fatal error", func(t *testing.T) {

		startCount := 0
		pm := func(l *slog.Logger) agentProcess {
			return mockAgent(func(ctx context.Context, config config.TPLAgent) error {
				startCount++
				return fatal.NewError(errors.New("fatal error"))
			})
		}

		cfgFileLocation := tmpDir + "/agent-config.json"
		f, err := os.OpenFile(cfgFileLocation, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
		if err != nil {
			t.Errorf("error creating config file:%v", err)
			return
		}

		cfg := makeConfig("test", tmpDir)
		if err := json.NewEncoder(f).Encode(cfg); err != nil {
			t.Error(err)
			return
		}

		err = spawnAndReload(context.Background(), pm, f.Name())
		if !fatal.Is(err) {
			t.Errorf("expected fatal error got %v", err)
		}
		if startCount > 1 {
			t.Error("agent reloaded")
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
