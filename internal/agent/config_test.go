package agent

import (
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"log/slog"
	"strings"
	"testing"
	"text/template"
	"time"
)

type sampleConfig struct {
	URL       string `json:"url"`
	AuthToken string `json:"auth_token"`
}
type sampleProvider struct {
	sc *sampleConfig
}

func (s *sampleProvider) FuncMap() template.FuncMap {
	return make(template.FuncMap)
}

func (s *sampleProvider) SetConfig(bb []byte) error {
	var sc sampleConfig
	err := json.Unmarshal(bb, &sc)
	if err != nil {
		return err
	}
	s.sc = &sc
	return nil
}

func Test_readConfig(t *testing.T) {
	t.Run("read config test", func(t *testing.T) {
		configJSON := `{
  "agent": {
    "log_level": "ERROR",
    "log_fmt": "json"
  },
  "templates": {
    "test-config": {
      "refresh_interval": "10s",
      "before_render_cmd": "echo \"hello\"",
      "after_render_cmd": "echo \"rendererd\"",
      "source": "/etc/tmpl/test.tmpl",
      "destination": "/etc/config/test.cfg",
      "providers": [
        {
          "name": "test_provider",
          "config": {
            "key": "val"
          }
        }
      ]
    },
    "test-config2": {
      "refresh_interval": "5s",
      "before_render_cmd": "echo \"hello\"",
      "after_render_cmd": "echo \"rendererd\"",
      "source": "/etc/tmpl/test.tmpl",
      "destination": "/etc/config/test.cfg"
    }
  }
}
`
		expectedConfig := Config{
			Agent: Agent{
				LogLevel: slog.LevelError,
				LogFmt:   "json",
			},
			Templates: map[string]Template{
				"test-config": {
					RefreshInterval: RefreshInterval(10 * time.Second),
					BeforeRenderCMD: `echo "hello"`,
					AfterRenderCMD:  `echo "rendererd"`,
					Source:          "/etc/tmpl/test.tmpl",
					Destination:     "/etc/config/test.cfg",
					Providers: []Provider{
						{
							Name: "test_provider",
							Config: json.RawMessage(`{
            "key": "val"
          }`),
						},
					},
				},
				"test-config2": {
					RefreshInterval: RefreshInterval(5 * time.Second),
					BeforeRenderCMD: `echo "hello"`,
					AfterRenderCMD:  `echo "rendererd"`,
					Source:          "/etc/tmpl/test.tmpl",
					Destination:     "/etc/config/test.cfg",
				},
			},
		}
		c, err := readConfig(strings.NewReader(configJSON))
		if err != nil {
			t.Errorf("error reading config:%v", err)
			return
		}

		if diff := cmp.Diff(expectedConfig, c); diff != "" {
			t.Errorf("(--Want ++Got):\n%s", diff)
		}
	})

	t.Run("test config validation", func(t *testing.T) {
		c := &Config{
			Agent: Agent{
				LogLevel: slog.LevelError,
				LogFmt:   "xml",
			},
			Templates: map[string]Template{
				"templ-conf": {
					RefreshInterval: RefreshInterval(500 * time.Millisecond),
					Providers:       []Provider{{}},
				},
				"templ-conf2": {
					RefreshInterval: RefreshInterval(1 * time.Second),
					Source:          "/tmpl/t.tmpl",
					Destination:     "/tmpl/dest",
				},
			},
		}
		if err := validateConfig(c); err == nil {
			t.Errorf("expected error got nil")
		} else {
			t.Log(err)
		}

	})

	t.Run("test sample provider config read", func(t *testing.T) {
		configJSON := `{
  "agent": {
    "log_level": "ERROR",
    "log_fmt": "json"
  },
  "templates": {
    "test-config": {
      "refresh_interval": "10s",
      "before_render_cmd": "echo \"hello\"",
      "after_render_cmd": "echo \"rendererd\"",
      "source": "/etc/tmpl/test.tmpl",
      "destination": "/etc/config/test.cfg",
      "providers": [
        {
          "name": "test_provider",
          "config": {
            "url": "http://some.domain.com:8100",
			"auth_token": "SECRET_123"
          }
        }
      ]
    }
  }
}
`
		conf, err := readConfig(strings.NewReader(configJSON))
		if err != nil {
			t.Errorf("error reading config:%v\n", err)
		}
		templConf := conf.Templates["test-config"]
		prov := templConf.Providers[0]
		sp := sampleProvider{}
		if err := sp.SetConfig(prov.Config); err != nil {
			t.Errorf("error reading config for sample provider:%v\n", err)
			return
		}
		expectedConfig := sampleConfig{
			URL:       "http://some.domain.com:8100",
			AuthToken: "SECRET_123",
		}
		if diff := cmp.Diff(expectedConfig, *sp.sc); diff != "" {
			t.Errorf("(-Want +Got):\n%s", diff)
		}
	})
}
