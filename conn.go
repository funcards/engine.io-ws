package eiows

import (
	"net"
	"net/url"
	"time"
)

type Conn struct {
	net.Conn

	Timeout time.Duration
	query   url.Values
	headers map[string]string
}

func (d Conn) Write(p []byte) (int, error) {
	if err := d.Conn.SetWriteDeadline(time.Now().Add(d.Timeout)); err != nil {
		return 0, err
	}
	return d.Conn.Write(p)
}

func (d Conn) Read(p []byte) (int, error) {
	if err := d.Conn.SetReadDeadline(time.Now().Add(d.Timeout)); err != nil {
		return 0, err
	}
	return d.Conn.Read(p)
}

func (d Conn) Name() string {
	return d.LocalAddr().String() + " > " + d.RemoteAddr().String()
}
