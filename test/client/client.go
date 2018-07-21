package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/CodeZeg/udp"
)

func main() {

	// 设置
	udp.Pack_max_len = 256
	udp.Kcp_mtu = 252
	udp.Sess_ch_socket_size = 64
	udp.Sess_ch_logic_size = 16

	die := make(chan bool, 1)
	for i := 8600; i < 8700; i++ {
		go func(conv uint32) {
			client, err := udp.NewClient(conv, "192.168.1.134:4000")
			if err != nil {
				fmt.Println(err)
			}

			index := 0
			timer := time.NewTicker(time.Millisecond * 1)
		CLOSED:
			for {
				select {
				case err := <-client.Err:
					fmt.Println("udp client error :" + err.Error())
				case <-timer.C:
					index++
					str := "hello " + strconv.Itoa(int(conv)) + " ___ " + strconv.Itoa(index)
					fmt.Println("send : " + str)
					client.Send([]byte(str))
					if index > 5000 {
						client.Close()
						break CLOSED
					}
				case buf := <-client.ChLogic:
					fmt.Println("recv : " + string(buf))
					client.PushBuf(buf)
				}
			}
			fmt.Println("client closed")
		}(uint32(i))
	}

	for {
		select {
		case <-die:
			break
		}
	}
}
