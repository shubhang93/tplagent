package tplactions

import (
	"fmt"
	"os"
)

const envPrefix = "TPL_AGENT"

func LookupEnv(key string) (string, bool) {
	k := fmt.Sprintf("%s_%s", envPrefix, key)
	return os.LookupEnv(k)
}
