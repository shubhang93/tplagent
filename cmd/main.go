package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	err := startCLI(ctx, os.Stdout, os.Args[1:]...)
	if err != nil && !isCtxErr(err) {
		_, _ = fmt.Fprintf(os.Stderr, "cmd failed with:%s", err.Error())
		os.Exit(1)
	}
}
