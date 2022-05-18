package eiows

import (
	"github.com/gobwas/ws"
	"io"
	"net/url"
	"strings"
)

type Upgrader struct {
	ws.Upgrader

	query   url.Values
	headers map[string]string
}

func (u *Upgrader) onHeader(key, value []byte) error {
	u.headers[strings.ToLower(string(key))] = string(value)
	return nil
}

func (u *Upgrader) onRequest(uri []byte) (err error) {
	u.query, err = url.ParseQuery(string(uri))
	return
}

func (u *Upgrader) Upgrade(conn io.ReadWriter) (hs ws.Handshake, err error) {
	u.headers = make(map[string]string)
	u.OnHeader = u.onHeader
	u.OnRequest = u.onRequest

	hs, err = u.Upgrader.Upgrade(conn)
	if err != nil {
		return
	}

	if safeConn, ok := conn.(*Conn); ok {
		safeConn.headers = u.headers
		safeConn.query = u.query
	}
	return
}
