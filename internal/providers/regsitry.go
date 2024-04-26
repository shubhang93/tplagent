package providers

import "fmt"

var Registry = map[string]MakeFunc{}

type MakeFunc func() Interface

func Register(name string, maker MakeFunc) {
	if _, ok := Registry[name]; ok {
		panic(fmt.Sprintf("provider %s already exists", name))
	}
	Registry[name] = maker
}
