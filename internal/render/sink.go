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
const copyBuffSize = 32 * 1024

var ContentsIdentical = errors.New("identical contents")

type executableTemplate interface {
	Execute(io.Writer, any) error
}

type Sink struct {
	Templ         executableTemplate
	WriteTo       string
	destFileBytes *bytes.Buffer
	copyBuffer    []byte
}

func (s *Sink) Render(staticData any) error {
	s.init()

	defer func() {
		clear(s.copyBuffer)
		s.destFileBytes.Reset()
	}()

	if err := ensureDestDirs(s.WriteTo); err != nil {
		return err
	}

	s.destFileBytes.Reset()
	if err := renderTempl(s.Templ, s.destFileBytes, staticData); err != nil {
		return err
	}

	oldFileContents, readErr := os.ReadFile(s.WriteTo)
	switch {
	case readErr == nil:
		if res := bytes.Compare(oldFileContents, s.destFileBytes.Bytes()); res == 0 {
			return ContentsIdentical
		}

		if err := atomicBackup(s.WriteTo, bytes.NewReader(oldFileContents), s.copyBuffer); err != nil {
			return fmt.Errorf("backup failed:%w", err)
		}

		if err := atomicWriteDest(s.WriteTo, s.destFileBytes, s.copyBuffer); err != nil {
			return fmt.Errorf("atomic write failed:%w", err)
		}

	case errors.Is(readErr, os.ErrNotExist):
		if err := atomicWriteDest(s.WriteTo, s.destFileBytes, s.copyBuffer); err != nil {
			return fmt.Errorf("atomic write failed:%w", err)
		}
	default:
		return readErr

	}
	return nil
}

func atomicBackup(dest string, contents io.Reader, copyBuff []byte) error {
	bakFilename := fmt.Sprintf("%s.%s", dest, bakFileExt)
	bakFile, err := createWritableFile(bakFilename)
	if err != nil {
		return err
	}
	defer bakFile.Close()

	clear(copyBuff)
	_, err = io.CopyBuffer(bakFile, contents, copyBuff)
	if err != nil {
		_ = os.Remove(bakFilename)
		return err
	}
	return nil
}

func atomicWriteDest(dest string, contents io.Reader, copyBuff []byte) error {
	tempFileName := fmt.Sprintf("%s.%s", dest, tempFileExt)
	tempFile, err := createWritableFile(tempFileName)
	if err != nil {
		return err
	}
	defer tempFile.Close()

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
	if s.destFileBytes == nil {
		s.destFileBytes = &bytes.Buffer{}
	}
	if s.copyBuffer == nil {
		s.copyBuffer = make([]byte, copyBuffSize)
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
