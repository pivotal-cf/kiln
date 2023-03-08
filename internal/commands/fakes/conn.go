package fakes

import (
	"io"
	"net"
	"time"
)

type Conn struct {
	R io.Reader
	W io.Writer
}

func (c *Conn) LocalAddr() net.Addr {
	return nil
}

func (c *Conn) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "test", Net: "test"}
}

func (c *Conn) SetDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *Conn) Read(p []byte) (int, error) {
	return c.R.Read(p)
}

func (c *Conn) Write(p []byte) (int, error) {
	return c.W.Write(p)
}

func (c *Conn) Close() error {
	return nil
}
