package graph

import (
	"os"
	"testing"
)

// GRAPH_BENCH is one text the benchmarks read.
func benchGraph(b *testing.B) []byte {
	f := os.Getenv("GRAPH_BENCH")
	if f == "" {
		b.Skip("GRAPH_BENCH is not set")
	}
	src, err := os.ReadFile(f)
	if err != nil {
		b.Fatal(err)
	}
	g, _, err := Import(f, src)
	if err != nil {
		b.Fatal(err)
	}
	return g.Lisp()
}

func BenchmarkRead(b *testing.B) {
	text := benchGraph(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Read(text); err != nil {
			b.Fatal(err)
		}
	}
}
