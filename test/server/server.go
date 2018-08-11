package main

import (
	"fmt"

	"github.com/CodeZeg/udp"
)

func main() {
	// 设置 因为参数数量有点多 所以全部使用变量而非函数 并且已经给了一个相对较好的默认值
	udp.Lis_sess_capacity = 1024
	udp.Lis_pool_size = 256
	udp.Pack_max_len = 256
	udp.Kcp_mtu = 252
	udp.Sess_ch_socket_size = 64
	udp.Sess_ch_logic_size = 16

	lis, err := udp.Listen(":4000")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("server started")

	for i := 6000; i < 10000; i++ {
		conv := uint32(i)
		s, err := lis.AddSession(conv)
		if err != nil {
			fmt.Println(err)
		}

		go func(s *udp.Session) {
			index := 0
		CLOSED:
			for {
				select {
				case err := <-s.Err:
					fmt.Println("udp session error :" + err.Error())
				case buf := <-s.ChLogic:
					fmt.Println("recv : " + string(buf))
					s.Send(buf)
					// 注意这里 收到的逻辑包处理完成之后需要扔回给session重用
					s.PushBuf(buf)

					index++
					if index > 10000 {
						// lis.RemoveSession(conv)
						lis.Close()
						break CLOSED
					}
				}
			}

		}(s)
	}

	for {
		if lis.Closed {
			break
		}

		select {
		case err := <-lis.Err:
			fmt.Println("udp listener error :" + err.Error())
		}
	}

	fmt.Println("udp listener closed")
}
