package udp

import (
	"sync"
)

// 网络数据缓存池
type bufpool struct {
	bufs [][]byte    // 数据列表
	size int         // 每个数据的容量
	lock *sync.Mutex // 锁(ps:这里的锁策略理论上是会影响到接收数据包的性能的,后面再考虑分类锁和去锁方案)
}

func newBufPool(capacity, size int) *bufpool {
	p := new(bufpool)
	p.bufs = make([][]byte, capacity)[:0]
	p.size = size
	p.lock = &sync.Mutex{}
	return p
}

func (p *bufpool) push(buf []byte) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.bufs = append(p.bufs, buf[:p.size])
}

func (p *bufpool) pop() []byte {
	p.lock.Lock()
	defer p.lock.Unlock()

	len := len(p.bufs)
	if len > 0 {
		ret := p.bufs[len-1]
		p.bufs = p.bufs[:len-1]
		return ret
	} else {
		return make([]byte, p.size)
	}
}
