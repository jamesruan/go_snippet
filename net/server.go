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
	l       net.Listener
	closed  chan struct{}
	closing chan chan error
}

type ConnHandler interface {
	HandleConn(net.Conn) error
}

func NewServer(args ServerArgs, handler ConnHandler) *Server {
	return &Server{
		ServerArgs:  args,
		ConnHandler: handler,
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

	accepted := s.doAccept()
	for {
		select {
		case reply := <-s.closing:
			close(s.closed)
			reply <- l.Close()
		case result := <-accepted:
			if result.err != nil {
				return result.err
			}
			go func() {
				defer result.conn.Close()
				if err := s.HandleConn(result.conn); err != nil {
					s.Logger.Printf("HandleConn: %s", err)
				}
			}()
			accepted = s.doAccept()
		}
	}
}

func (s *Server) doAccept() <-chan acceptResult {
	tempDelay := 5 * time.Millisecond
	ch := make(chan acceptResult)
	go func() {
		for {
			conn, err := s.l.Accept()
			if err != nil {
				select {
				case <-s.closed:
					ch <- acceptResult{nil, ErrServerClosed}
					return
				default:
				}
				// fast retry start from 5 milliseconds when temporary error
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					if max := 1 * time.Second; tempDelay > max {
						tempDelay = max
					}
					s.Logger.Printf("accept: %s, retry in %s", ne, tempDelay)
					time.Sleep(tempDelay)
					tempDelay *= 2
					continue
				}
				close(s.closed)
				s.l.Close()
				ch <- acceptResult{nil, err}
				return
			}
			ch <- acceptResult{conn, nil}
			return
		}
	}()
	return ch
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
