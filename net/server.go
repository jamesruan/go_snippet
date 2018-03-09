package net

import "errors"
import "os"
import "log"
import "net"
import "time"
import "sync"
import "context"

var ErrServerClose = errors.New("net: server closed")

type ServerArgs struct {
	ListenNetwork string
	ListenAddr    string
	MinTempDelay  time.Duration
	MaxTempDelay  time.Duration
	*log.Logger
}

func NewServerArgs() *ServerArgs {
	a := new(ServerArgs)
	a.ListenAddr = "[::1]:0"
	a.ListenNetwork = "tcp"
	a.MinTempDelay = 5 * time.Millisecond
	a.MaxTempDelay = 5 * time.Second
	a.Logger = log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)
	return a
}

type Server struct {
	ServerArgs
	h      ConnHandler
	ctx    context.Context
	cancel context.CancelFunc
	mu     *sync.Mutex
	l      net.Listener
}

type ConnHandler func(Server, net.Conn)

func NewServer(args ServerArgs, h ConnHandler) *Server {
	return &Server{
		ServerArgs: args,
		h:          h,
		mu:         new(sync.Mutex),
	}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	l, err := net.Listen(s.ListenNetwork, s.ListenAddr)
	if err != nil {
		return err
	}
	defer l.Close()
	nctx, cancel := context.WithCancel(ctx)
	s.ctx = nctx
	s.cancel = cancel

	var tempDelay time.Duration
	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return ErrServerClose
			default:
			}
			// fast retry start from 5 milliseconds when temporary error
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if tempDelay > 1*time.Second {
					tempDelay = s.MaxTempDelay
				}
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0
		s.h(*s, conn)
	}
}
