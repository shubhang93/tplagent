package agent

import (
	"github.com/shubhang93/tplagent/internal/tplactions"
	"testing"
	"unicode"
)

func TestFunctionNames(t *testing.T) {

	for actionName, _ := range tplactions.Registry {
		if !goodName(actionName) {
			t.Errorf("action name %s is invalid", actionName)
			return
		}
	}

}

// taken from text/template code
func goodName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_':
		case i == 0 && !unicode.IsLetter(r):
			return false
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			return false
		}
	}
	return true
}
