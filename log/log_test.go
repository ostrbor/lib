package log

import "testing"

func BenchmarkLog(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Log("No context.")
		}
	})
}
