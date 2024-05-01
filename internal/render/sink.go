package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const tempFileExt = "temp"
const bakFileExt = "bak"
const mode = os.FileMode(0766)

type executableTemplate interface {
	Execute(io.Writer, any) error
}

type Sink struct {
	Templ        executableTemplate
	WriteTo      string
	fileContents *bytes.Buffer
	copyBuffer   []byte
}

func (s *Sink) Render(staticData any) error {
	s.init()

	defer func() {
		clear(s.copyBuffer)
		s.fileContents.Reset()
	}()

	if err := ensureDestDirs(s.WriteTo); err != nil {
		return err
	}

	tempFileName := fmt.Sprintf("%s.%s", s.WriteTo, tempFileExt)
	tempFile, err := createWritableFile(tempFileName)
	if err != nil {
		return err
	}

	s.fileContents.Reset()
	if err := renderTempl(s.Templ, s.fileContents, staticData); err != nil {
		return err
	}
	oldFileContents, err := os.ReadFile(s.WriteTo)
	switch {
	case err == nil:
		if bytes.Equal(oldFileContents, s.fileContents.Bytes()) {
			return nil
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return fmt.Errorf("error reading old file contents:%w", err)
	}

	oldFile, err := os.Open(s.WriteTo)

	switch {
	case err == nil:
		bakFileName := fmt.Sprintf("%s.%s", s.WriteTo, bakFileExt)
		bakFile, err := createWritableFile(bakFileName)
		if err != nil {
			return err
		}
		err = makeBackup(bakFile, oldFile, s.copyBuffer)
		if err != nil {
			return fmt.Errorf("failed to backup file:%w", err)
		}

		if err := writeTempFile(tempFile, s.fileContents, s.copyBuffer); err != nil {
			return fmt.Errorf("error writing to temp file:%w", err)
		}

		if err := os.Rename(tempFile.Name(), s.WriteTo); err != nil {
			return fmt.Errorf("error renaming file:%w", err)
		}
	case errors.Is(err, os.ErrNotExist):
		if err := writeTempFile(tempFile, s.fileContents, s.copyBuffer); err != nil {
			return fmt.Errorf("error writing to temp file:%w", err)
		}
		if err := os.Rename(tempFile.Name(), s.WriteTo); err != nil {
			return fmt.Errorf("error renaming file:%w", err)
		}
	default:
		return err
	}
	return nil
}

func makeBackup(bakFile *os.File, oldFile *os.File, buff []byte) error {
	clear(buff)
	_, err := io.CopyBuffer(bakFile, oldFile, buff)
	if err != nil {
		return err
	}
	return nil
}

func writeTempFile(tempFile *os.File, contents *bytes.Buffer, buff []byte) error {
	clear(buff)
	if _, err := io.CopyBuffer(tempFile, contents, buff); err != nil {
		return err
	}
	return nil
}

func (s *Sink) init() {
	if s.fileContents == nil {
		s.fileContents = &bytes.Buffer{}
	}
	if s.copyBuffer == nil {
		s.copyBuffer = make([]byte, 4096)
	}
}

func ensureDestDirs(filename string) error {
	dirPath := filepath.Dir(filename)
	err := os.MkdirAll(dirPath, mode)
	if err != nil {
		return fmt.Errorf("failed to create dir path:%s:%w", dirPath, err)
	}
	err = os.Chmod(dirPath, mode)
	if err != nil {
		return fmt.Errorf("failed to change perms on dir:%s:%w", dirPath, err)
	}

	return nil
}

func createWritableFile(filename string) (*os.File, error) {
	fi, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(fi.Name(), mode); err != nil {
		return nil, err
	}
	return fi, nil
}

func renderTempl(t executableTemplate, wr io.Writer, staticData any) error {
	if err := t.Execute(wr, staticData); err != nil {
		return fmt.Errorf("error writing dest file:%w", err)
	}
	return nil
}
