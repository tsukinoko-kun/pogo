package config

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/gliderlabs/ssh"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Username  string `yaml:"username"`
	PublicKey string `yaml:"public_key"`
}

var config *Config

func getConfig() *Config {
	configFileName := getConfigFileName()
	if config == nil {
		_, err := os.Stat(configFileName)
		if err != nil {
			u, _ := user.Current()
			config = &Config{
				Username: u.Username,
			}
			f, _ := os.Create(configFileName)
			defer f.Close()
			ye := yaml.NewEncoder(f)
			ye.SetIndent(4)
			_ = ye.Encode(config)
			fmt.Fprintln(os.Stderr, "created config file in "+configFileName)
			return config
		} else {
			f, _ := os.Open(configFileName)
			defer f.Close()
			config = &Config{}
			_ = yaml.NewDecoder(f).Decode(config)
			return config
		}
	}
	return config
}

func getConfigFileName() string {
	loc := getConfigLocation()
	os.MkdirAll(loc, 0755)
	return filepath.Join(loc, "config.yaml")
}

func GetConfigFileName() string {
	_ = getConfig()
	return getConfigFileName()
}

func GetUsername() string {
	return getConfig().Username
}

func GetPublicKey() (ssh.PublicKey, bool) {
	pk := strings.TrimSpace(getConfig().PublicKey)
	if len(pk) == 0 {
		return nil, false
	}
	if key, err := ssh.ParsePublicKey([]byte(pk)); err == nil {
		return key, true
	}
	if key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pk)); err == nil {
		return key, true
	}
	if _, err := os.Stat(pk); err == nil {
		// it is a file path, try to read it
		f, err := os.Open(pk)
		if err == nil {
			defer f.Close()
			pkBytes, err := io.ReadAll(f)
			if err == nil {
				if key, err := ssh.ParsePublicKey(pkBytes); err == nil {
					return key, true
				}
				if key, _, _, _, err := ssh.ParseAuthorizedKey(pkBytes); err == nil {
					return key, true
				}
			}
		}
	}
	return nil, false
}
