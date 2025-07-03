package utils

import (
	"crypto/sha256"
	"io"
	"os"
)

func Ptr[T any](v T) *T {
	return &v
}

func HashFile(absPath string) []byte {
	f, err := os.Open(absPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	return HashReader(f)
}

func HashReader(r io.Reader) []byte {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil
	}
	return h.Sum(nil)
}

func GetFileSize(absPath string) (int64, error) {
	stat, err := os.Stat(absPath)
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}
