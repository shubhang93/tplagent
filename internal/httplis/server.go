package httplis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/shubhang93/tplagent/internal/config"
	"log/slog"
	"net/http"
	"os"
)

type reloadRequest struct {
	Config     config.TPLAgent `json:"config"`
	ConfigPath string          `json:"config_path"`
}

func Start(ctx context.Context, addr string, l *slog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /config/reload", func(writer http.ResponseWriter, request *http.Request) {
		reloadReq := reloadRequest{}
		err := json.NewDecoder(request.Body).Decode(&reloadReq)
		if err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		_, err = os.Stat(reloadReq.ConfigPath)
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(writer, http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("file not found at %s", reloadReq.ConfigPath),
			})
			return
		}

		err = config.Validate(&reloadReq.Config)
		if err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

	})

	s := http.Server{Handler: mux, Addr: addr}

	wait := make(chan struct{})
	go func() {
		defer close(wait)
		_ = s.Shutdown(ctx)
	}()

	err := s.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) && err != nil {
		l.Error("ListenAndServe error", slog.String("error", err.Error()))
	}

	<-wait

}

func writeJSON(writer http.ResponseWriter, status int, data any) {
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(data)
	return
}
