package repos

import (
	"encoding/base64"
	"github.com/tsukinoko-kun/pogo/utils"
	"io"
	"os"
	"path/filepath"
)

func (r Repo) ContentHashToFileName(contentHash []byte) string {
	str := base64.RawURLEncoding.EncodeToString(contentHash)
	return filepath.Join("content", str[:2], str[2:])
}

func (r Repo) FileExists(contentHash []byte) (bool, error) {
	name := r.ContentHashToFileName(contentHash)
	_, err := os.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r Repo) GetFileInfo(contentHash []byte) (os.FileInfo, error) {
	name := r.ContentHashToFileName(contentHash)
	return os.Stat(name)
}

func (r Repo) GetFileContent(contentHash []byte) (io.ReadCloser, error) {
	name := r.ContentHashToFileName(contentHash)
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (r Repo) SetFileContent(contentHash []byte, content io.Reader) error {
	name := r.ContentHashToFileName(contentHash)
	_ = os.MkdirAll(filepath.Dir(name), 0755)
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, utils.Compress(content))
	return err
}
