package agent

import (
	"encoding/json"
	"fmt"
	"github.com/shubhang93/tplagent/internal/duration"
	"io"
	"log/slog"
	"time"
)

func WriteConfig(wr io.Writer, numBlocks int) error {
	starter := generateConfig(numBlocks)

	jd := json.NewEncoder(wr)
	jd.SetIndent("", " ")
	if err := jd.Encode(starter); err != nil {
		return err
	}
	return nil
}

func generateConfig(numBlocks int) Config {
	starter := Config{
		Agent: AgentConfig{
			LogLevel:               slog.LevelInfo,
			LogFmt:                 "text",
			MaxConsecutiveFailures: 10,
		},
		TemplateSpecs: map[string]*TemplateConfig{},
	}

	for i := range numBlocks {
		num := i + 1
		tplBlock := makeTemplBlock(num)
		starter.TemplateSpecs[fmt.Sprintf("myapp-config%d", num)] = tplBlock
	}
	return starter
}

func makeTemplBlock(i int) *TemplateConfig {
	return &TemplateConfig{
		Actions:     []ActionsConfig{},
		Source:      fmt.Sprintf("/path/to/template-file%d", i),
		Destination: fmt.Sprintf("/path/to/outfile%d", i),
		StaticData: map[string]string{
			"key": "value",
		},
		RefreshInterval: duration.Duration(1 * time.Second),
		RenderOnce:      false,
		MissingKey:      "error",
		Exec: &ExecConfig{
			Cmd:        "echo",
			CmdArgs:    []string{"hello"},
			CmdTimeout: duration.Duration(30 * time.Second),
		},
	}
}
