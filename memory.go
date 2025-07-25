package main

import "sync"

func NewBytePool() *BytePool {
	return &BytePool{
		pool: sync.Pool{
			New: func() any {
				// Start with 1KB buffers, will grow as needed
				return make([]byte, 1024)
			},
		},
	}
}

func (bp *BytePool) Get(size int) []byte {
	buf := bp.pool.Get().([]byte)
	if cap(buf) < size {
		// If buffer is too small, create a new one
		return make([]byte, size)
	}
	return buf[:size]
}

// func (bp *BytePool) Put(buf []byte) {
// 	if cap(buf) <= 64*1024 { // Don't pool very large buffers
// 		bp.pool.Put(buf)
// 	}
// }

func (bp *BytePool) Put(buf []byte) {
	if cap(buf) <= 64*1024 { // Don't pool very large buffers
		// Reset slice to zero length but preserve capacity
		buf = buf[:0]
		bp.pool.Put(buf)
	}
}
