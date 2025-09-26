package buffer

import (
	"sync"
)

type Buffer []int16

type Pool struct {
	pool sync.Pool
}

func NewPool(size int) *Pool {
	return &Pool{
		pool: sync.Pool{
			New: func() any {
				b := make(Buffer, size)
				return &b
			},
		},
	}
}

func (pool *Pool) Get() *Buffer {
	return pool.pool.Get().(*Buffer)
}

func (pool *Pool) Put(b *Buffer) {
	pool.pool.Put(b)
}
