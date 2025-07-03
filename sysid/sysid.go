package sysid

import (
	"crypto/sha256"
	"encoding/base64"
)

const salt = "pogopopo"

var machineId string

func GetMachineID() (string, error) {
	if machineId != "" {
		return machineId, nil
	}
	if s, err := getMachineID(); err == nil {
		h := sha256.New()
		h.Write([]byte(salt))
		h.Write([]byte(s))
		machineId = base64.StdEncoding.EncodeToString(h.Sum(nil))
		return machineId, nil
	} else {
		return "", err
	}
}

func MustGetMachineID() string {
	if machineId != "" {
		return machineId
	}
	if s, err := getMachineID(); err == nil {
		h := sha256.New()
		h.Write([]byte(salt))
		h.Write([]byte(s))
		machineId = base64.StdEncoding.EncodeToString(h.Sum(nil))
		return machineId
	} else {
		panic(err)
	}
}
