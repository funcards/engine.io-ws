package eiows

import (
	"fmt"
	"github.com/funcards/engine.io"
	"github.com/funcards/engine.io-parser/v4"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"go.uber.org/zap"
	"net/url"
	"sync"
)

var _ eio.WebSocket = (*webSocket)(nil)

type webSocket struct {
	eio.Emitter

	io   sync.Mutex
	conn *Conn
	log  *zap.Logger
}

func NewWebSocket(conn *Conn, logger *zap.Logger) *webSocket {
	return &webSocket{
		Emitter: eio.NewEmitter(),
		conn:    conn,
		log:     logger,
	}
}

func (s *webSocket) Query() url.Values {
	return s.conn.query
}

func (s *webSocket) Headers() map[string]string {
	return s.conn.headers
}

func (s *webSocket) Close() error {
	s.io.Lock()
	defer s.io.Unlock()

	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			s.conn = nil
			return err
		}
		s.conn = nil
		s.log.Debug("websocket: conn closed")
	}
	return nil
}

func (s *webSocket) Write(packet eiop.Packet) {
	s.log.Debug("websocket: write packet", zap.Any("packet", packet))

	switch packet.Data.(type) {
	case []byte, string, nil:
		data := packet.Encode(true)
		go s.write(data)
	default:
		s.log.Warn("websocket: type of packet data is not valid", zap.Any("packet", packet))
	}
}

func (s *webSocket) OnClose(reason, description string) {
	_ = s.Close()

	s.log.Info(fmt.Sprintf("websocket: %s", reason), zap.String("description", description))
	s.Fire(eio.TopicClose, reason, description)
}

func (s *webSocket) OnRead() bool {
	packet, err := s.read()
	if err != nil {
		s.onError("websocket: read error", err)
		return false
	}
	s.Fire(eio.TopicPacket, packet)

	return true
}

func (s *webSocket) read() (eiop.Packet, error) {
	var packet eiop.Packet

	s.io.Lock()
	defer s.io.Unlock()

	data, op, err := wsutil.ReadClientData(s.conn)
	if err != nil {
		if _, ok := err.(wsutil.ClosedError); ok {
			return packet, fmt.Errorf("peer has closed the connection: %w", err)
		}
		return packet, fmt.Errorf("read client data: %w", err)
	}

	isText := op == ws.OpText
	s.log.Debug("websocket: read data", zap.Bool("is_text", isText), zap.Binary("data", data))

	if isText {
		packet, err = eiop.DecodePacket(string(data))
	} else {
		packet, err = eiop.DecodePacket(data)
	}
	if err != nil {
		return packet, fmt.Errorf("decode data: %w", err)
	}

	return packet, nil
}

func (s *webSocket) write(data any) {
	s.io.Lock()
	defer s.io.Unlock()

	if s.conn == nil {
		s.log.Warn("websocket: write on closed conn")
		return
	}

	s.log.Debug("websocket: write data", zap.Any("data", data))

	switch tmp := data.(type) {
	case string:
		if err := wsutil.WriteServerText(s.conn, []byte(tmp)); err != nil {
			s.onError("websocket: conn write text", err)
		}
	case []byte:
		if err := wsutil.WriteServerBinary(s.conn, tmp); err != nil {
			s.onError("websocket: conn write binary", err)
		}
	}
}

func (s *webSocket) onError(msg string, err error) {
	_ = s.Close()

	s.log.Warn(msg, zap.Error(err))
	s.Fire(eio.TopicError, msg, err)
}
