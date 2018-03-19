package net

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"runtime"
	"time"
)

type ClientArgs struct {
	RemoteNetwork string
	RemoteAddr    string
	*log.Logger
}

var (
	ErrClosedConn = errors.New("connecting a closed client")
	ErrOpenedConn = errors.New("connecting an opened client")
)

func MakeClientArgs(network, addr string) ClientArgs {
	return ClientArgs{
		RemoteNetwork: network,
		RemoteAddr:    addr,
		Logger:        log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile),
	}
}

type Client struct {
	ClientArgs
	ConnHandler
	conn    net.Conn
	closed  chan struct{}
	closing chan chan error
}

func NewClient(args ClientArgs, handler ConnHandler) *Client {
	c := &Client{
		ClientArgs:  args,
		ConnHandler: handler,
		closed:      make(chan struct{}),
		closing:     make(chan chan error),
	}
	// in case that Close() is not called
	runtime.SetFinalizer(c, func(o *Client) {
		o.Close()
	})
	return c
}

func (c *Client) Connect(ctx context.Context) error {
	var tempDelay time.Duration
	for {
		select {
		case <-c.closed:
			return ErrClosedConn
		default:
			if c.conn != nil {
				return ErrOpenedConn
			}
		}
		dialer := &net.Dialer{}
		conn, err := dialer.DialContext(ctx, c.RemoteNetwork, c.RemoteAddr)
		if err != nil {
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
				c.Logger.Printf("connect: %s", ne)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		c.conn = conn
		go func() {
			reply := <-c.closing
			close(c.closed)
			reply <- conn.Close()
		}()
		return c.HandleConn(conn)
	}
}

func (c *Client) Close() error {
	err := make(chan error)
	select {
	case c.closing <- err:
		return <-err
	default:
		return nil
	}
}
