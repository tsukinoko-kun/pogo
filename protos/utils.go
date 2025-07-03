package protos

import (
	"bytes"
	"io"

	"google.golang.org/protobuf/proto"
)

func Unmarshal(r io.Reader, m proto.Message) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return proto.Unmarshal(b, m)
}

func Marshal(m proto.Message) io.Reader {
	if m == nil {
		return nil
	}
	b, err := proto.Marshal(m)
	if err != nil {
		return bytes.NewReader(nil)
	}
	return bytes.NewReader(b)
}

func MarshalWrite(m proto.Message, w io.Writer) error {
	b, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}
