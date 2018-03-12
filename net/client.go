package net

import "os"
import "log"
import "net"
import "time"

type ClientArgs struct {
	RemoteNetwork string
	RemoteAddr    string
	*log.Logger
}

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
	conn   net.Conn
	closed chan struct{}
}

func NewClient(args ClientArgs) *Client {
	return &Client{
		ClientArgs: args,
		closed:     make(chan struct{}),
	}
}

//Blocks until handler is done with the conn
func (c *Client) Connect() error {
	var tempDelay time.Duration
	for {
		conn, err := net.Dial(c.RemoteNetwork, c.RemoteAddr)
		if err != nil {
			select {
			case <-c.closed:
				return nil
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
				c.Logger.Printf("connect: %s", ne)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		c.conn = conn
		defer conn.Close()
		return c.HandleConn(conn)
	}
}

func (c *Client) HandleConn(net.Conn) error {
	c.Logger.Printf("not implemented")
	return nil
}

func (c *Client) Close() error {
	select {
	case <-c.closed:
		return nil
	default:
		return c.conn.Close()
	}
}
