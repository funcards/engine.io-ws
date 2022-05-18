package eiows

import (
	"github.com/funcards/engine.io"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"go.uber.org/zap"
	"net/url"
	"sync"
)

type IOWebSocket struct {
	eio.Emitter

	io   sync.Mutex
	conn *Conn
	log  *zap.Logger
}

func NewIOWebSocket(conn *Conn, logger *zap.Logger) *IOWebSocket {
	return &IOWebSocket{
		Emitter: eio.NewEmitter(logger),
		conn:    conn,
		log:     logger,
	}
}

func (s *IOWebSocket) GetQuery() url.Values {
	return s.conn.query
}

func (s *IOWebSocket) GetHeaders() map[string]string {
	return s.conn.headers
}

func (s *IOWebSocket) Write(data any) error {
	go func(data any) {
		s.io.Lock()
		defer s.io.Unlock()

		s.log.Debug("ws conn write", zap.Any("data", data))

		switch tmp := data.(type) {
		case string:
			if err := wsutil.WriteServerText(s.conn, []byte(tmp)); err != nil {
				s.log.Warn("websocket write text", zap.Error(err))
				if err = s.Emit(eio.TopicError, "websocket write", err.Error()); err != nil {
					s.log.Warn("websocket emit", zap.Error(err))
				}
			}
		case []byte:
			if err := wsutil.WriteServerBinary(s.conn, tmp); err != nil {
				s.log.Warn("websocket write binary", zap.Error(err))
				if err = s.Emit(eio.TopicError, "websocket write", err.Error()); err != nil {
					s.log.Warn("websocket emit", zap.Error(err))
				}
			}
		}
	}(data)
	return nil
}

func (s *IOWebSocket) Close() error {
	s.io.Lock()
	defer s.io.Unlock()

	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}

func (s *IOWebSocket) OnClose() error {
	s.io.Lock()
	defer s.io.Unlock()

	return s.Emit(eio.TopicClose)
}

func (s *IOWebSocket) Receive() error {
	s.io.Lock()
	data, op, err := wsutil.ReadClientData(s.conn)
	s.io.Unlock()

	if err != nil {
		if closed, ok := err.(wsutil.ClosedError); ok {
			s.log.Info("peer has closed the connection", zap.String("reason", closed.Reason), zap.Error(closed))
		}
		return err
	}

	isText := op == ws.OpText

	s.log.Debug("ws received", zap.Bool("is_text", isText), zap.Binary("data", data))

	if isText {
		return s.Emit(eio.TopicMessage, string(data))
	} else {
		return s.Emit(eio.TopicMessage, data)
	}
}
