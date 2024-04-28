package render

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type executableTemplate interface {
	Execute(io.Writer, any) error
}

type Sink struct {
	Templ      executableTemplate
	WriteTo    string
	buffWriter *bufio.Writer
	scratch    *bytes.Buffer
}

func (s *Sink) Render(staticData any) error {
	s.init()
	return backupAndRender(s.Templ, s.WriteTo, s.buffWriter, staticData)
}

func (s *Sink) init() {
	if s.buffWriter == nil {
		s.buffWriter = bufio.NewWriter(nil)
	}
	if s.scratch == nil {
		s.scratch = bytes.NewBuffer(make([]byte, 2048))
	}
}

func backupAndRender(t executableTemplate, writeTo string, buffWr *bufio.Writer, staticData any) error {
	err := backupOldFileIfExists(writeTo)
	if err != nil {
		return fmt.Errorf("could not create backup for %s:%w", writeTo, err)
	}

	fi, err := createDest(writeTo)
	if err != nil {
		return fmt.Errorf("could not create dest file for %s:%w", writeTo, err)
	}
	defer fi.Close()

	buffWr.Reset(fi)
	return renderTempl(t, buffWr, staticData)
}

func backupOldFileIfExists(filename string) error {
	_, err := os.Stat(filename)
	if !errors.Is(err, os.ErrNotExist) {
		bakFilename := filename + ".bak"
		err := os.Rename(filename, bakFilename)
		if err != nil {
			return err
		}
		_ = os.Chmod(bakFilename, 0644)
	}
	return nil
}

func createDest(filename string) (*os.File, error) {
	dirPath := filepath.Dir(filename)
	err := os.MkdirAll(dirPath, 0755)

	if err != nil {
		return nil, fmt.Errorf("failed to create dir path:%s:%w", dirPath, err)
	}

	return os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
}

func renderTempl(t executableTemplate, buffWr *bufio.Writer, staticData any) error {
	defer buffWr.Flush()
	if err := t.Execute(buffWr, staticData); err != nil {
		return fmt.Errorf("error writing dest file:%w", err)
	}
	return nil
}
