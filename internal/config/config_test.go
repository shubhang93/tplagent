package config

import (
	"bytes"
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"github.com/shubhang93/tplagent/internal/duration"
	"github.com/shubhang93/tplagent/internal/fatal"
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
type sampleActions struct {
	sc *sampleConfig
}

func (s *sampleActions) FuncMap() template.FuncMap {
	return make(template.FuncMap)
}

func (s *sampleActions) SetConfig(bb []byte) error {
	var sc sampleConfig
	err := json.Unmarshal(bb, &sc)
	if err != nil {
		return err
	}
	s.sc = &sc
	return nil
}

func Test_readConfig(t *testing.T) {
	t.Run("Read config test", func(t *testing.T) {
		configJSON := `{
  "agent": {
    "log_level": "ERROR",
    "log_fmt": "json",
	"max_consecutive_failures": 25,
	"http_listener_addr": "localhost:5000"
  },
  "templates": {
    "test-config": {
      "refresh_interval": "10s",
      "exec_cmd": "echo \"rendererd\"",
      "exec": {
        "cmd": "echo",
        "cmd_args": [
          "rendered"
        ]
      },
      "source": "/etc/tmpl/test.tmpl",
      "destination": "/etc/config/test.cfg",
      "actions": [
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
      "exec": {
        "cmd": "echo",
        "cmd_args": [
          "rendered"
        ],
		"env": {"HOME": "xyzzy/spoonshift1"}
      },
      "source": "/etc/tmpl/test.tmpl",
      "destination": "/etc/config/test.cfg"
    }
  }
}
`
		expectedConfig := TPLAgent{
			Agent: Agent{
				LogLevel:               slog.LevelError,
				LogFmt:                 "json",
				MaxConsecutiveFailures: 25,
				HTTPListenerAddr:       "localhost:5000",
			},
			TemplateSpecs: map[string]*TemplateSpec{
				"test-config": {
					RefreshInterval: duration.Duration(10 * time.Second),
					Exec: &ExecSpec{
						Cmd:     "echo",
						CmdArgs: []string{"rendered"},
					},
					Source:      "/etc/tmpl/test.tmpl",
					Destination: "/etc/config/test.cfg",
					Actions: []Actions{
						{
							Name: "test_provider",
							Config: json.RawMessage(`{
            "key": "val"
          }`),
						},
					},
				},
				"test-config2": {
					RefreshInterval: duration.Duration(5 * time.Second),
					Exec: &ExecSpec{
						Cmd:     "echo",
						CmdArgs: []string{"rendered"},
						Env:     map[string]string{"HOME": "xyzzy/spoonshift1"},
					},
					Source:      "/etc/tmpl/test.tmpl",
					Destination: "/etc/config/test.cfg",
				},
			},
		}
		c, err := Read(strings.NewReader(configJSON))
		if err != nil {
			t.Errorf("error reading config:%v", err)
			return
		}

		if diff := cmp.Diff(expectedConfig, c); diff != "" {
			t.Errorf("(--Want ++Got):\n%s", diff)
		}
	})

	t.Run("test config validation", func(t *testing.T) {
		c := &TPLAgent{
			Agent: Agent{
				LogLevel: slog.LevelError,
				LogFmt:   "xml",
			},
			TemplateSpecs: map[string]*TemplateSpec{
				"templ-conf": {
					RefreshInterval: duration.Duration(500 * time.Millisecond),
					Actions:         []Actions{{}},
				},
				"templ-conf2": {
					RefreshInterval: duration.Duration(1 * time.Second),
					Source:          "/tmpl/parsed.tmpl",
					Destination:     "/tmpl/dest",
				},
			},
		}
		if err := Validate(c); err == nil {
			t.Errorf("expected error got nil")
		} else {
			t.Log(err)
		}

	})

	t.Run("test sample provider config Read", func(t *testing.T) {
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
      "actions": [
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
		conf, err := Read(strings.NewReader(configJSON))
		if err != nil {
			t.Errorf("error reading config:%v\n", err)
		}
		templConf := conf.TemplateSpecs["test-config"]
		prov := templConf.Actions[0]
		sp := sampleActions{}
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

func Test_config_fatalErrors(t *testing.T) {
	t.Run("ReadConfig fails", func(t *testing.T) {
		_, err := ReadFromFile("/some/path/that/does/not/exist")
		if !fatal.Is(err) {
			t.Error("expected fatal error")
		}
	})

	t.Run("config decoding fails", func(t *testing.T) {
		var buff bytes.Buffer
		err := WriteTo(&buff, 1, 2)
		if err != nil {
			t.Error(err)
			return
		}

		// mangle the json
		jsonCfg := buff.Bytes()
		i := bytes.Index(jsonCfg, []byte{'{'})
		jsonCfg[i] = '['
		buff.Reset()
		buff.Write(jsonCfg)

		_, err = Read(&buff)
		if !fatal.Is(err) {
			t.Error("expected fatal error")
			return
		}

	})
	t.Run("config validation fails", func(t *testing.T) {
		var buff bytes.Buffer
		err := WriteTo(&buff, 1, 2)
		if err != nil {
			t.Error(err)
			return
		}

		var c TPLAgent
		err = json.NewDecoder(&buff).Decode(&c)
		if err != nil {
			t.Error(err)
			return
		}

		for _, spec := range c.TemplateSpecs {
			spec.RefreshInterval = duration.Duration(500 * time.Millisecond)
		}

		buff.Reset()
		err = json.NewEncoder(&buff).Encode(c)
		if err != nil {
			t.Error(err)
			return
		}

		_, err = Read(&buff)
		if !fatal.Is(err) {
			t.Error("expected fatal error")
		}

	})

}
