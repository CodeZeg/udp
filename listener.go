package udp

import (
	"errors"
	"net"
	"strconv"
	"sync"
)

var (
	Lis_sess_capacity  int = 1024 // 监听器的初始容量
	Channel_Pack_count int = 256  // 监听器的解包分发协程数量
	Channel_Pack_size  int = 1024 // 监听器的收到通道数量
)

type Listener struct {
	conn     net.UDPConn     // udp连接
	Err      chan error      // 错误通道 具体错误处理交给逻辑层
	sessions sync.Map        // uint32 - *Session // 会话列表
	closed   bool            // 关闭状态
	chClosed chan bool       // 关闭通道 用于关闭内部协程
	chPack   []chan inPacket // 收包通道列表
}

type inPacket struct {
	addr net.Addr
	buf  []byte
}

// 监听指定地址
func Listen(laddr string) (*Listener, error) {
	udpaddr, err := net.ResolveUDPAddr("udp", laddr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		return nil, err
	}

	l := new(Listener)
	l.conn = *conn
	l.Err = make(chan error, 32)
	l.closed = false
	l.chClosed = make(chan bool)

	go l.receiver()

	l.chPack = make([]chan inPacket, Channel_Pack_count)
	for i := 0; i < Channel_Pack_count; i++ {
		l.chPack[i] = make(chan inPacket, Channel_Pack_size)
		go l.monitor(l.chPack[i])
	}

	return l, nil
}

func (l *Listener) receiver() {
	count := 0
	for {
		if l.closed {
			break
		}

		buf := make([]byte, Pack_max_len)
		n, addr, err := l.conn.ReadFrom(buf)
		if err != nil {
			l.Err <- err
			break
		} else {
			l.chPack[count] <- inPacket{addr, buf[:n]}
			count++
			if count == Channel_Pack_count {
				count = 0
			}
		}
	}
}

// 收到消息的处理
func (l *Listener) monitor(chPack <-chan inPacket) {
	for {
		select {
		case <-l.chClosed:
			return
		case pack := <-chPack:
			var conv uint32
			ikcp_decode32u(pack.buf[fecHeaderSize:], &conv)
			s, ok := l.sessions.Load(conv)
			if !ok {
				l.Err <- errors.New("收到不存在的会话发来的消息 : " + strconv.Itoa(int(conv)))
			} else {
				s.(*Session).remote_addr = pack.addr
				s.(*Session).chSocket <- pack.buf
			}
		}
	}
}

// 添加会话
func (l *Listener) AddSession(conv uint32) (*Session, error) {
	_, ok := l.sessions.Load(conv)
	if ok {
		err := errors.New("had the same seesion id" + strconv.Itoa(int(conv)))
		return nil, err
	}

	s := newSession(conv, l.conn)
	l.sessions.Store(conv, s)
	return s, nil
}

// 移除会话
func (l *Listener) RemoveSession(conv uint32) {
	s, ok := l.sessions.Load(conv)
	if !ok {
		l.Err <- errors.New("not find seesion id" + strconv.Itoa(int(conv)))
	}

	s.(*Session).close()
	l.sessions.Delete(conv)
}

// 关闭
func (l *Listener) Close() {
	l.closed = true

	for i := 0; i < Channel_Pack_count; i++ {
		l.chClosed <- true
	}

	l.sessions.Range(func(key, value interface{}) bool {
		value.(*Session).close()
		l.sessions.Delete(key.(uint32))

		return true
	})
	l.conn.Close()
}
