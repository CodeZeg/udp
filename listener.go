package udp

import (
	"errors"
	"net"
	"strconv"
	"sync"
)

var (
	Lis_sess_capacity int = 1024 // 监听器的初始容量
	Lis_ch_pack_size  int = 128  // 接收socket包的通道大小
)

type Listener struct {
	conn net.UDPConn // udp连接
	Err  chan error  // 错误通道 具体错误处理交给逻辑层

	pool *bufpool // 数据包缓存池 所有session公用一个
	//sessions map[uint32]*Session // 会话列表
	sessions sync.Map      // uint32 - *Session // 会话列表
	chPack   chan inPacket // 数据包通道
	Closed   bool          // 关闭状态
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
	//l.sessions = make(map[uint32]*Session, Lis_sess_capacity)
	l.pool = newBufPool(Pack_pool_size, Pack_max_len)
	l.chPack = make(chan inPacket, Lis_ch_pack_size)
	l.Closed = false

	go l.receiver(l.chPack)
	go l.monitor(l.chPack)

	return l, nil
}

func (l *Listener) receiver(chPack chan<- inPacket) {
	for {
		if l.Closed {
			break
		}

		buf := l.pool.pop()
		n, addr, err := l.conn.ReadFrom(buf)
		if err != nil {
			l.Err <- err
			break
		} else {
			chPack <- inPacket{addr, buf[:n]}
		}
	}
}

// 监听socket收到的消息
func (l *Listener) monitor(chPack <-chan inPacket) {
CLOSED:
	for {
		if l.Closed {
			break CLOSED
		}

		select {
		case pack := <-chPack:
			var conv uint32
			ikcp_decode32u(pack.buf[fecHeaderSize:], &conv)
			//s, ok := l.sessions[conv]
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

	s := newSession(conv, l.conn, l.pool)
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
	l.Closed = true

	l.sessions.Range(func(key, value interface{}) bool {
		value.(*Session).close()
		l.sessions.Delete(key.(uint32))

		return true
	})
	// l.conn.Close()
}
