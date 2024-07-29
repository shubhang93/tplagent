package agent

import (
	"fmt"
	"github.com/shubhang93/tplagent/internal/actionable"
	"github.com/shubhang93/tplagent/internal/config"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"log/slog"
	"os"
	"strings"
	"text/template"
)

const agentEnvPrefix = "TPLA"

func attachActions(t *actionable.Template, registry map[string]tplactions.MakeFunc, l *slog.Logger, templActions []config.Actions) error {
	namesSpacedFuncMap := make(template.FuncMap)
	for _, ta := range templActions {
		actionMaker, ok := registry[ta.Name]
		if !ok {
			return fmt.Errorf("invalid action name:%s", ta.Name)
		}
		action := actionMaker()
		if err := action.SetConfig(ta.Config, tplactions.SetConfigOpts{
			EnvPrefix: makeEnvPrefix(t.Name),
		}); err != nil {
			return fmt.Errorf("error setting config for %s:%w", ta.Name, err)
		}
		action.SetLogger(l)
		t.AddAction(action)
		fm := action.FuncMap()
		for name, f := range fm {
			funcNameWithNS := []byte(ta.Name)
			funcNameWithNS = append(funcNameWithNS, '_')
			funcNameWithNS = append(funcNameWithNS, name...)
			namesSpacedFuncMap[string(funcNameWithNS)] = f
		}
	}
	// template.Funcs validates
	// function names in the FuncMap
	// we cannot use special chars
	// except underscores
	// we prefix the action name
	// to each of the function names
	// ex: api_getJSON
	t.Funcs(namesSpacedFuncMap)
	return nil
}

func makeEnvPrefix(tmplName string) string {
	sanitizedName := strings.ToUpper(strings.ReplaceAll(tmplName, "-", "_"))
	return fmt.Sprintf("%s_%s", agentEnvPrefix, sanitizedName)
}

func setTemplateDelims(t *actionable.Template, delims []string) {
	if len(delims) < 2 {
		return
	}
	left, right := delims[0], delims[1]
	t.Delims(left, right)
}

func parseTemplate(raw string, readFrom string, pt *actionable.Template) error {
	if raw != "" {
		return pt.Parse(raw)
	}
	expandedPath := readFrom
	bs, err := os.ReadFile(expandedPath)
	if err != nil {
		return err
	}

	return pt.Parse(string(bs))
}
