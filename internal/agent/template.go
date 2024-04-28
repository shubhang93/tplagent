package agent

import (
	"fmt"
	"github.com/shubhang93/tplagent/internal/tplactions"
	"text/template"
)

type actionableTemplate interface {
	Funcs(actions map[string]any)
}

func attachActions(t actionableTemplate, templActions []ActionConfig) error {
	namesSpacedFuncMap := make(template.FuncMap)
	for _, ta := range templActions {
		actionMaker, ok := tplactions.Registry[ta.Name]
		if !ok {
			return fmt.Errorf("invalid action name:%s", ta.Name)
		}
		action := actionMaker()
		if err := action.SetConfig(ta.Config); err != nil {
			return fmt.Errorf("error setting config for %s", ta.Name)
		}
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

type delimitableTemplate interface {
	Delims(l, r string)
}

func setTemplateDelims(t delimitableTemplate, delims []string) {
	if len(delims) < 2 {
		return
	}
	left, right := delims[0], delims[1]
	t.Delims(left, right)
}
