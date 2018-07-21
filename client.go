package udp

import (
	"net"

	"github.com/pkg/errors"
)

type Client struct {
	conn    net.PacketConn // socket
	session *Session       // 会话
	pool    *bufpool       // 数据缓存
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
	c.pool = newBufPool(256, 1024)
	c.session = newSession(conv, *conn, c.pool)
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

		buf := c.pool.pop()
		n, _, err := c.conn.ReadFrom(buf)
		if c.Closed {
			c.pool.push(buf)
			break CLOSED
		} else if err != nil {
			c.pool.push(buf)
			c.Err <- err
		} else {
			buf = buf[:n]
			c.session.chSocket <- buf
		}
	}
}

// 逻辑层已经处理完数据之后需要手动在这里缓存数据 重复利用
func (c *Client) PushBuf(buf []byte) {
	c.session.PushBuf(buf)
}

// 关闭
func (c *Client) Close() {
	c.Closed = true
	c.session.close()
	c.conn.Close()
}
