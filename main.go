package dht

import (
	"fmt"
	"runtime"
)

const (
	idSize  = 20
	mapSize = 1 << 32
)

func main() {
	e, p, ms, m := 0, 1, runtime.MemStats{}, map[string]struct{}{}
	runtime.ReadMemStats(&ms)
	fmt.Printf("% 2d: %d", e, ms.HeapInuse)
	for i := 1; i <= mapSize; i++ {
		s := fmt.Sprintf("%020d\n", i)
		m[s] = struct{}{}
		if i == p {
			p <<= 1
			e++
			runtime.ReadMemStats(&ms)
			fmt.Printf("%02d: %d\n", e, ms.HeapInuse)
		}
	}
}
