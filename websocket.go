package eiows

import (
	"context"
	"errors"
	"github.com/funcards/engine.io"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"go.uber.org/zap"
	"net/url"
	"sync"
)

var ClosedError = errors.New("IOWebSocket closed")

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

func (s *IOWebSocket) Write(ctx context.Context, data any) {
	go func(ctx context.Context, data any) {
		s.io.Lock()
		defer s.io.Unlock()

		s.log.Debug("ws conn write", zap.Any("data", data))

		switch tmp := data.(type) {
		case string:
			if err := wsutil.WriteServerText(s.conn, []byte(tmp)); err != nil {
				s.log.Warn("websocket write text", zap.Error(err))
				s.Emit(ctx, eio.TopicError, "websocket write", err.Error())
			}
		case []byte:
			if err := wsutil.WriteServerBinary(s.conn, tmp); err != nil {
				s.log.Warn("websocket write binary", zap.Error(err))
				s.Emit(ctx, eio.TopicError, "websocket write", err.Error())
			}
		}
	}(ctx, data)
}

func (s *IOWebSocket) Close(ctx context.Context) {
	s.io.Lock()
	defer s.io.Unlock()

	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			s.log.Warn("io websocket close", zap.Error(err))
		}
		s.conn = nil
		eio.TryCancel(ctx, ClosedError)
	}
}

func (s *IOWebSocket) OnClose(ctx context.Context) {
	s.io.Lock()
	defer s.io.Unlock()

	s.Emit(ctx, eio.TopicClose)
}

func (s *IOWebSocket) Receive(ctx context.Context) {
	s.io.Lock()
	data, op, err := wsutil.ReadClientData(s.conn)
	s.io.Unlock()

	if err != nil {
		if closed, ok := err.(wsutil.ClosedError); ok {
			s.log.Info("peer has closed the connection", zap.String("reason", closed.Reason), zap.Error(closed))
		}
		eio.TryCancel(ctx, err)
		return
	}

	isText := op == ws.OpText

	s.log.Debug("ws received", zap.Bool("is_text", isText), zap.Binary("data", data))

	if isText {
		s.Emit(ctx, eio.TopicMessage, string(data))
	} else {
		s.Emit(ctx, eio.TopicMessage, data)
	}
}
