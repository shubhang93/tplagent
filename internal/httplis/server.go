package httplis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/shubhang93/tplagent/internal/config"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
)

type reloadRequest struct {
	Config     config.TPLAgent `json:"config"`
	ConfigPath string          `json:"config_path"`
}

type Proc struct {
	Logger   *slog.Logger
	Reloaded bool
}

const reloadEndpoint = "POST /config/reload"
const stopAgent = "POST /agent/stop"

func (p *Proc) Start(ctx context.Context, addr string) {
	mux := http.NewServeMux()

	mux.HandleFunc(reloadEndpoint, p.reloadConfig)
	mux.HandleFunc(stopAgent, p.stopAgent)

	srvr := http.Server{Handler: mux, Addr: addr}

	wait := make(chan struct{})
	go func() {
		defer close(wait)

		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = srvr.Shutdown(shutdownCtx)
	}()

	if p.Reloaded {
		p.Logger.Info("reloading http listener", slog.String("addr", addr))
	} else {
		p.Logger.Info("starting http listener", slog.String("addr", addr))
	}

	err := srvr.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) && err != nil {
		p.Logger.Error("ListenAndServe error", slog.String("error", err.Error()))
	}

	<-wait
	p.Logger.Info("http listener exited without errors")

}

func (p *Proc) stopAgent(writer http.ResponseWriter, _ *http.Request) {
	p.Logger.Info("stopping agent", slog.String("cause", "http stop triggerred"))
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	err = proc.Signal(syscall.SIGINT)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(writer, http.StatusOK, map[string]bool{"success": true})

}

func writeJSON(writer http.ResponseWriter, status int, data any) {
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(data)
	return
}

func (p *Proc) reloadConfig(writer http.ResponseWriter, request *http.Request) {

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	reloadReq := reloadRequest{}
	err = json.NewDecoder(request.Body).Decode(&reloadReq)
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	configFilePath := reloadReq.ConfigPath
	_, err = os.Stat(configFilePath)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(writer, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("file not found at %p", configFilePath),
		})
		return
	}

	err = config.Validate(&reloadReq.Config)
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	err = backupAndReplace(configFilePath, reloadReq.Config)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	p.Logger.Info("wrote new config")

	err = proc.Signal(syscall.SIGHUP)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(writer, http.StatusOK, map[string]bool{"success": true})

}

func backupAndReplace(path string, newConfig config.TPLAgent) error {
	bakFilename := fmt.Sprintf("%s.%s", path, "bak")
	bakFile, err := os.Create(bakFilename)
	if err != nil {
		return err
	}

	oldFile, err := os.Open(path)
	if err != nil {
		return err
	}

	_, err = io.Copy(bakFile, oldFile)
	if err != nil {
		return err
	}
	_ = bakFile.Close()

	tempFilename := fmt.Sprintf("%s.%s", path, "temp")
	tempFile, err := os.Create(tempFilename)
	if err != nil {
		_ = os.Remove(bakFilename)
		return err
	}

	jd := json.NewEncoder(tempFile)
	jd.SetIndent("", strings.Repeat(" ", 2))
	err = jd.Encode(newConfig)
	if err != nil {
		_ = os.Remove(tempFilename)
		return err
	}

	err = os.Rename(tempFilename, path)
	if err != nil {
		_ = os.Remove(tempFilename)
		return err
	}
	return nil

}
