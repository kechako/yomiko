package buffer

import "testing"

func TestFramePool(t *testing.T) {
	const size = 1024
	pool := NewPool(size)

	b := pool.Get()
	if got := len(*b); got != size {
		t.Errorf("Pool.Get(): buffer size: got %v, want %v", got, size)
	}

	pool.Put(b)
	b2 := pool.Get()
	if b2 != b {
		t.Error("Pool.Get(): did not store to pool")
	}
}
