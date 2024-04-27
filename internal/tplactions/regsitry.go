package tplactions

import "fmt"

var Registry = map[string]MakeFunc{}

type MakeFunc func() Interface

func Register(name string, maker MakeFunc) {
	if _, ok := Registry[name]; ok {
		panic(fmt.Sprintf("action %s already exists", name))
	}
	Registry[name] = maker
}
