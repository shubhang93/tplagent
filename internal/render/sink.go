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

var ContentsIdentical = errors.New("identical contents")

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

	s.fileContents.Reset()
	if err := renderTempl(s.Templ, s.fileContents, staticData); err != nil {
		return err
	}

	oldFileContents, readErr := os.ReadFile(s.WriteTo)
	switch {
	case readErr == nil:
		if res := bytes.Compare(oldFileContents, s.fileContents.Bytes()); res == 0 {
			return ContentsIdentical
		}

		bakFile := fmt.Sprintf("%s.bak", s.WriteTo)
		if err := os.WriteFile(bakFile, oldFileContents, mode); err != nil {
			return fmt.Errorf("failed to create backup:%w", err)
		}

		if err := atomicWrite(s.WriteTo, s.fileContents, s.copyBuffer); err != nil {
			return fmt.Errorf("atomic write failed:%w", err)
		}

	case errors.Is(readErr, os.ErrNotExist):
		if err := atomicWrite(s.WriteTo, s.fileContents, s.copyBuffer); err != nil {
			return fmt.Errorf("atomic write failed:%w", err)
		}
	default:
		return readErr

	}

	return nil
}

func atomicWrite(dest string, contents io.Reader, copyBuff []byte) error {
	tempFileName := fmt.Sprintf("%s.%s", dest, tempFileExt)
	tempFile, err := createWritableFile(tempFileName)
	if err != nil {
		return err
	}

	clear(copyBuff)
	if err := writeTempFile(tempFile, contents, copyBuff); err != nil {
		return fmt.Errorf("error writing to temp file:%w", err)
	}
	if err := os.Rename(tempFile.Name(), dest); err != nil {
		_ = os.Remove(tempFileName)
		return fmt.Errorf("error renaming file:%w", err)
	}
	return nil
}

func writeTempFile(tempFile *os.File, contents io.Reader, buff []byte) error {
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
	fi, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, mode)
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
