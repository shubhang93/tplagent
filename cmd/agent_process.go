package main

import (
	"context"
	"fmt"
	"github.com/shubhang93/tplagent/internal/agent"
	"os"
	"os/signal"
	"syscall"
)

type semToken struct{}

func main() {

}

func printUsage() {
	fmt.Println(`Usage:
tplagent start -config=/path/to/config.json`)
}

func spawnAndReload(parent context.Context) {
	ctx, cancel := createContext(parent)
	defer cancel()
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	defer func() {
		signal.Reset(syscall.SIGHUP)
	}()

	// only one agent goroutine can run
	sem := make(chan semToken, 1)

	acquireSem(sem)
	go func() {
		spawn(ctx, agent.Run, "")
		releaseSem(sem)
	}()

	run := true
	for run {
		select {
		case <-sighup:
			cancel()
			// wait for the old
			// agent goroutine to
			// release the sem
			<-sem
			// reset the context
			ctx, cancel = createContext(context.Background())
			// spawn a new agent to reload config
			acquireSem(sem)
			go func() {
				spawn(ctx, agent.Run, "")
				releaseSem(sem)
			}()
		case <-ctx.Done():
			cancel()
			run = false
		}
	}

	<-sem
}

func spawn(ctx context.Context, runFunc func(context.Context, agent.Config) error, configPath string) {
	config, err := agent.ReadConfigFromFile(configPath)
	if err != nil {
		fmt.Println("error reading config:", err.Error())
		return
	}
	err = runFunc(ctx, config)
	if err != nil {
		fmt.Println("agent error:", err.Error())
	}
	return
}

func acquireSem(sem chan semToken) {
	sem <- semToken{}
}

func releaseSem(sem chan semToken) {
	<-sem
}

func createContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGTERM, syscall.SIGINT)
}
