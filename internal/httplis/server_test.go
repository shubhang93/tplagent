package httplis

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestStart(t *testing.T) {

	type reloadTest struct {
		name       string
		wantStatus int
		jsonBody   func(string) string
		beforeFunc func(string) error
		wantSIGHUP bool
	}

	reloadTests := []reloadTest{
		{
			name:       "config path does not exist",
			wantStatus: http.StatusNotFound,
			jsonBody: func(_ string) string {
				return `{
  "config_path": "/some/path"
}`
			},
		},
		{
			name: "invalid config",
			beforeFunc: func(tmp string) error {
				_, err := os.Create(tmp + "/config.json")
				return err
			},
			wantStatus: http.StatusBadRequest,
			jsonBody: func(tmp string) string {
				return fmt.Sprintf(`{
  "config_path": "%s",
  "config": {
    "agent": {
      "log_fmt": "invalid"
    }
  }
}`, tmp+"/config.json")
			},
		},
		{
			name:       "valid config",
			wantStatus: http.StatusOK,
			jsonBody: func(tmp string) string {
				return fmt.Sprintf(`{
  "config_path": "%s",
  "config": {
    "agent": {
      "log_fmt": "text",
      "log_level": "INFO",
      "http_listener": "localhost:5000"
    },
    "templates": {
      "server-conf": {
        "raw": "hello {{.name}}"
      }
    }
  }
}`, tmp+"/config.json")
			},

			beforeFunc: func(tmp string) error {
				_, err := os.Create(tmp + "/config.json")
				return err
			},
			wantSIGHUP: true,
		},
	}

	for _, rt := range reloadTests {
		t.Run(rt.name, func(t *testing.T) {

			tmp := t.TempDir()
			if rt.beforeFunc != nil {
				if err := rt.beforeFunc(tmp); err != nil {
					t.Errorf("before func run error:%v", err)
					return
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			var wg sync.WaitGroup
			sighup := make(chan os.Signal)
			signal.Notify(sighup, syscall.SIGHUP)
			sighupRcvd := false

			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case <-ctx.Done():
				case <-sighup:
					sighupRcvd = true
				}
			}()

			server := Server{Logger: newLogger()}

			wg.Add(1)
			go func() {
				defer wg.Done()
				server.Start(ctx, "localhost:6000")
			}()

			time.Sleep(100 * time.Millisecond)
			rdr := strings.NewReader(rt.jsonBody(tmp))
			resp, err := http.Post("http://localhost:6000/config/reload", "application/json", rdr)
			if err != nil {
				t.Errorf("POST error:%v", err)
				return
			}
			respBody, _ := io.ReadAll(resp.Body)
			t.Log(string(respBody))
			if resp.StatusCode != rt.wantStatus {
				t.Errorf("expected status to be %d got %d", rt.wantStatus, resp.StatusCode)
				return
			}

			wg.Wait()
			if rt.wantSIGHUP != sighupRcvd {
				t.Error("SIGHUP not received")
			}

		})
	}

}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
