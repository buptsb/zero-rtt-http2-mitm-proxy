package internal

import (
	"io"

	"github.com/kelindar/binary"
)

type StreamType int

const (
	StreamTypeNormal StreamType = iota
	StreamTypePrefetch
)

type HandshakeMsg struct {
	StreamType StreamType
}

func (m *HandshakeMsg) WriteTo(w io.Writer) error {
	return binary.MarshalTo(m, w)
}

func UnmarshalHandshakeMsg(r io.Reader) (*HandshakeMsg, error) {
	var m HandshakeMsg
	dec := binary.NewDecoder(r)
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}
