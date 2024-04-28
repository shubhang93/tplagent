package render

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const tempFileExt = "temp"
const bakFileExt = "bak"

type executableTemplate interface {
	Execute(io.Writer, any) error
}

type Sink struct {
	Templ      executableTemplate
	WriteTo    string
	buffWriter *bufio.Writer
	copyBuffer []byte
}

func (s *Sink) Render(staticData any) error {
	s.init()

	if err := ensureDestDirs(s.WriteTo); err != nil {
		return err
	}

	tempFileName := fmt.Sprintf("%s.%s", s.WriteTo, tempFileExt)
	tempFile, err := createWritableFile(tempFileName)
	if err != nil {
		return err
	}

	s.buffWriter.Reset(tempFile)
	if err := renderTempl(s.Templ, s.buffWriter, staticData); err != nil {
		return err
	}

	oldFile, err := os.Open(s.WriteTo)

	switch {
	case err == nil:
		clear(s.copyBuffer)

		bakFileName := fmt.Sprintf("%s.%s", s.WriteTo, bakFileExt)
		bakFile, err := createWritableFile(bakFileName)
		if err != nil {
			return err
		}
		_, err = io.CopyBuffer(bakFile, oldFile, s.copyBuffer)
		if err != nil {
			return fmt.Errorf("failed to backup file:%w", err)
		}
		if err := os.Rename(tempFile.Name(), s.WriteTo); err != nil {
			return fmt.Errorf("error renaming file:%w", err)
		}
	case errors.Is(err, os.ErrNotExist):
		if err := os.Rename(tempFile.Name(), s.WriteTo); err != nil {
			return fmt.Errorf("error renaming file:%w", err)
		}
	default:
		return err
	}
	return nil
}

func (s *Sink) init() {
	if s.buffWriter == nil {
		s.buffWriter = bufio.NewWriter(nil)
	}
	if s.copyBuffer == nil {
		s.copyBuffer = make([]byte, 4096)
	}
}

func ensureDestDirs(filename string) error {
	dirPath := filepath.Dir(filename)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create dir path:%s:%w", dirPath, err)
	}
	return nil
}

func createWritableFile(filename string) (*os.File, error) {
	return os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
}

func renderTempl(t executableTemplate, buffWr *bufio.Writer, staticData any) error {
	defer buffWr.Flush()
	if err := t.Execute(buffWr, staticData); err != nil {
		return fmt.Errorf("error writing dest file:%w", err)
	}
	return nil
}
