package net

import (
	"errors"
	"log"
	"net"
	"os"
	"time"
)

var ErrServerClosed = errors.New("net: server closed")

type ServerArgs struct {
	ListenNetwork string
	ListenAddr    string
	*log.Logger
}

func MakeServerArgs(network, addr string) ServerArgs {
	return ServerArgs{
		ListenNetwork: network,
		ListenAddr:    addr,
		Logger:        log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile),
	}
}

type acceptResult struct {
	conn net.Conn
	err  error
}

type Server struct {
	ServerArgs
	ConnHandler
	l           net.Listener
	startAccept chan struct{}
	accepted    chan acceptResult
	closed      chan struct{}
	closing     chan chan error
}

type ConnHandler interface {
	HandleConn(net.Conn) error
}

func NewServer(args ServerArgs) *Server {
	return &Server{
		ServerArgs:  args,
		startAccept: make(chan struct{}, 1),
		accepted:    make(chan acceptResult),
		closed:      make(chan struct{}),
		closing:     make(chan chan error),
	}
}

//Blocks until accepting error
func (s *Server) ListenAndServe() error {
	l, err := net.Listen(s.ListenNetwork, s.ListenAddr)
	if err != nil {
		return err
	}
	s.l = l

	defer s.l.Close()

	s.startAccept <- struct{}{}

	for {
		select {
		case reply := <-s.closing:
			close(s.closed)
			reply <- l.Close()
		case <-s.startAccept:
			go s.doAccept()
		case result := <-s.accepted:
			if result.err != nil {
				select {
				case <-s.closed:
				default:
					close(s.closed)
				}
				return result.err
			}
			go func() {
				defer result.conn.Close()
				if err := s.HandleConn(result.conn); err != nil {
					s.Logger.Printf("HandleConn: %s", err)
				}
			}()
			s.startAccept <- struct{}{}
		}
	}
}

func (s *Server) doAccept() {
	var tempDelay time.Duration
	for {
		conn, err := s.l.Accept()
		if err != nil {
			select {
			case <-s.closed:
				s.accepted <- acceptResult{nil, ErrServerClosed}
				return
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
				s.Logger.Printf("accept: %s", ne)
				time.Sleep(tempDelay)
				continue
			}
			s.accepted <- acceptResult{nil, err}
			return
		}
		s.accepted <- acceptResult{conn, err}
		return
	}
}

func (s *Server) HandleConn(net.Conn) error {
	s.Logger.Printf("not implemented")
	return nil
}

func (s *Server) Close() error {
	err := make(chan error)
	select {
	case s.closing <- err:
		return <-err
	default:
		return nil
	}
}
