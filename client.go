package udp

import (
	"net"

	"github.com/pkg/errors"
)

type Client struct {
	conn    net.PacketConn // socket
	session *Session       // 会话
	Closed  bool

	Err     chan error  // 错误通道
	ChLogic chan []byte // 抛出数据给逻辑层的通道
}

func NewClient(conv uint32, raddr string) (*Client, error) {
	udpaddr, err := net.ResolveUDPAddr("udp", raddr)
	if err != nil {
		return nil, errors.Wrap(err, "net.ResolveUDPAddr")
	}

	conn, err := net.DialUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0}, udpaddr)
	if err != nil {
		return nil, errors.Wrap(err, "net.DialUDP")
	}

	c := new(Client)
	c.conn = conn
	c.session = newSession(conv, *conn)
	c.session.remote_addr = nil
	c.Err = c.session.Err
	c.ChLogic = c.session.ChLogic
	c.Closed = false

	go c.monitor()

	return c, nil
}

// 发送数据
func (c *Client) Send(buf []byte) {
	c.session.Send(buf)
}

// 监听socket收到的消息
func (c *Client) monitor() {
CLOSED:
	for {
		if c.Closed {
			break
		}

		buf := make([]byte, 1024)
		n, _, err := c.conn.ReadFrom(buf)
		if c.Closed {
			break CLOSED
		} else if err != nil {
			c.Err <- err
		} else {
			buf = buf[:n]
			c.session.chSocket <- buf
		}
	}
}

// 关闭
func (c *Client) Close() {
	c.Closed = true
	c.session.close()
	c.conn.Close()
}
