package udp

import (
	"errors"
	"net"
	"strconv"
	"time"
)

var (
	Pack_max_len int = 1024 // 单个udp包的最大包长，用于做缓存（ 必须 >= kcp_mtu + fec_head_Len）

	Fec_len      int    = 3   // fec 冗余册数 3 => p1(x1, x2, x3)
	Fec_cacheLen uint32 = 256 // fec 序号收到的记录长度 需要大于 kcp的wnd_size

	Kcp_nodelay  int           = 1   // kcp的延迟发送
	Kcp_interval int           = 33  // kcp的刷新间隔
	Kcp_resend   int           = 2   // kcP的重传次数
	Kcp_nc       int           = 1   // kcp的流控
	Kcp_mtu      int           = 256 // kcp输出的单个包的长度
	Kcp_wnd_size int           = 32  // kcp的窗口的大小
	Kcp_update   time.Duration = 10  // kcp的update调用时间间隔

	Sess_ch_socket_size int = 64 // 会话接收socket数据的通道大小
	Sess_ch_logic_size  int = 16 // 会话投递数据给logic的通道大小
)

// 数据流
// send2kcp: kcp=>fec=>socket
// recv2fec: socket=>fec=>kcp
// 缓存池流
// recv: pop=>chSocket=>fec=>kcp=>push
// seg: pop=>new=>delete=>push
// unpack: pop=>unpack=>logic=>push
type Session struct {
	conv     uint32      // 会话id
	kcp      *KCP        // kcp
	encoder  *fecEncoder // fec
	decoder  *fecDecoder // fec
	closed   bool        // 关闭
	chClosed chan bool   // 关闭通道 用于关闭协程

	conn        net.UDPConn // socket
	remote_addr net.Addr    // 远端地址,客户端的地址是可以随时变动的 因为wifi/4g等情况很常见
	chSocket    chan []byte // 接收socket数据的通道

	Err     chan error  // 错误信息通道
	ChLogic chan []byte // 抛出数据给逻辑层的通道
}

func newSession(conv uint32, conn net.UDPConn) *Session {
	s := new(Session)
	s.conv = conv
	s.conn = conn
	s.closed = false
	s.chClosed = make(chan bool)
	// fec
	s.encoder = newFECEncoder(Fec_len, Pack_max_len)
	s.decoder = newFECDecoder(Fec_cacheLen)
	// kcp
	s.kcp = NewKCP(conv, s.writeToFecToSocket)
	s.kcp.NoDelay(Kcp_nodelay, Kcp_interval, Kcp_resend, Kcp_nc)
	s.kcp.SetMtu(Kcp_mtu)
	s.kcp.WndSize(Kcp_wnd_size, Kcp_wnd_size)

	// 从网络层接收数据
	s.chSocket = make(chan []byte, Sess_ch_socket_size)
	// 抛数据给逻辑层
	s.ChLogic = make(chan []byte, Sess_ch_logic_size)
	// 异常处理
	s.Err = make(chan error, 1)

	go s.update()

	return s
}

// GetConv get conv id
func (s *Session) GetConv() uint32 {
	return s.conv
}

// send data to socket
func (s *Session) writeToFecToSocket(buf []byte, size int) {
	if s.closed {
		return
	}

	ecc := s.encoder.encode(buf[:size])
	for _, b := range ecc {
		if len(b) == 0 {
			break
		}

		if len(b) > Pack_max_len {
			s.Err <- errors.New("发送的数据量过大 :" + strconv.Itoa(len(b)))
			break
		}

		// dial 之后不能使用 writeto
		if s.remote_addr == nil {
			_, err := s.conn.Write(b)
			if err != nil {
				s.Err <- err
				break
			}
		} else {
			_, err := s.conn.WriteTo(b, s.remote_addr)
			if err != nil {
				s.Err <- err
				break
			}
		}
	}
}

// Send data to kcp
func (s *Session) Send(b []byte) {
	if !s.closed {
		s.kcp.Send(b)
	}
}

// 监听收取消息
func (s *Session) update() {
	timer := time.NewTicker(time.Millisecond * Kcp_update)
	for {
		select {
		case <-s.chClosed:
			return
		case data := <-s.chSocket:
			f := s.decoder.decodeBytes(data)
			if f != nil {
				err := s.kcp.Input(f, true, false)
				if err != 0 {
					s.Err <- errors.New("kcp recv data error " + strconv.Itoa(err))
				} else {
					for {
						if size := s.kcp.PeekSize(); size > 0 {
							buf := make([]byte, Pack_max_len)
							len := s.kcp.Recv(buf)
							s.ChLogic <- buf[:len]
						} else {
							break
						}
					}
				}
			}
		case <-timer.C:
			s.kcp.Update(uint32(time.Now().UnixNano() / int64(time.Millisecond)))
		}
	}
}

// 关闭会话
func (s *Session) close() {
	s.closed = true
	s.chClosed <- true
}
