package config

import (
	"encoding/json"
	"fmt"
	"github.com/shubhang93/tplagent/internal/duration"
	"io"
	"log/slog"
	"strings"
	"time"
)

func WriteTo(wr io.Writer, numBlocks int, indent int) error {
	starter := generate(numBlocks)

	jd := json.NewEncoder(wr)
	jd.SetIndent("", strings.Repeat(" ", indent))
	if err := jd.Encode(starter); err != nil {
		return err
	}
	return nil
}

func generate(numBlocks int) TPLAgent {
	starter := TPLAgent{
		Agent: Agent{
			LogLevel:               slog.LevelInfo,
			LogFmt:                 "text",
			MaxConsecutiveFailures: 10,
		},
		TemplateSpecs: map[string]*TemplateSpec{},
	}

	for i := range numBlocks {
		num := i + 1
		tplBlock := makeTemplBlock(num)
		starter.TemplateSpecs[fmt.Sprintf("myapp-config%d", num)] = tplBlock
	}
	return starter
}

func makeTemplBlock(i int) *TemplateSpec {
	return &TemplateSpec{
		Actions:     []Actions{},
		Source:      fmt.Sprintf("/path/to/template-file%d", i),
		Destination: fmt.Sprintf("/path/to/outfile%d", i),
		StaticData: map[string]string{
			"key": "value",
		},
		RefreshInterval: duration.Duration(1 * time.Second),
		RenderOnce:      false,
		MissingKey:      "error",
		Exec: &ExecSpec{
			Cmd:        "echo",
			CmdArgs:    []string{"hello"},
			CmdTimeout: duration.Duration(30 * time.Second),
		},
	}
}
