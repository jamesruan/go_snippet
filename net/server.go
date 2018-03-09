package net

import "errors"
import "os"
import "log"
import "net"
import "time"
import "context"

var ErrServerClose = errors.New("net: server closed")

type ServerArgs struct {
	ListenNetwork string
	ListenAddr    string
	*log.Logger
}

func NewServerArgs(network, addr string) *ServerArgs {
	a := new(ServerArgs)
	a.ListenNetwork = network
	a.ListenAddr = addr
	a.Logger = log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)
	return a
}

type Server struct {
	ServerArgs
	h      ConnHandler
	ctx    context.Context
	cancel context.CancelFunc
}

type ConnHandler func(Server, net.Conn)

func NewServer(args ServerArgs, h ConnHandler) *Server {
	return &Server{
		ServerArgs: args,
		h:          h,
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
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
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
