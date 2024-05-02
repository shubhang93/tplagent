package agent

import (
	"encoding/json"
	"io"
	"log/slog"
	"time"
)

func GenerateConfig(wr io.Writer) error {
	jd := json.NewEncoder(wr)
	jd.SetIndent("", " ")
	if err := jd.Encode(StarterConfig); err != nil {
		return err
	}
	return nil
}

var StarterConfig = Config{
	Agent: AgentConfig{
		LogLevel:               slog.LevelInfo,
		LogFmt:                 "text",
		MaxConsecutiveFailures: 50,
	},
	TemplateSpecs: map[string]*TemplateConfig{
		"myapp-config": {
			Actions:     []ActionsConfig{},
			Source:      "/path/to/template-file",
			Destination: "/path/to/outfile",
			StaticData: map[string]string{
				"key": "value",
			},
			RefreshInterval: Duration(1 * time.Second),
			RenderOnce:      false,
			MissingKey:      "error",
			Exec: &ExecConfig{
				Cmd:        "echo",
				CmdArgs:    []string{"hello"},
				CmdTimeout: Duration(30 * time.Second),
			},
		},
	},
}
