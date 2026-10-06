// Package rt is Go's runtime as Joker sees it, the namespace rt: the
// collector's work and the heap, for measuring the box
// (doc/LISP-SANDBOX.md, *The collector in the box*).
//
//	(rt/mem)   {:heap-mb :heap-sys-mb :sys-mb :gcs :pause-total-ms
//	            :pause-p50-us :pause-p99-us :pause-max-us :cpus :gc-cpu-pct}
//	(rt/gc)    a collection now; nil
//	(rt/trace-start)  Go's execution tracer started, into memory
//	(rt/trace-stop)   stopped: the trace, as a string of its bytes (for
//	                  box/put, then `go tool trace` on the host)
//
// The pauses are runtime/metrics' stop-the-world pauses of the collector
// (/sched/pauses/total/gc:seconds), a histogram since the program began;
// the quantiles are its buckets' upper bounds.
package rt

import (
	"bytes"
	"math"
	"runtime"
	"runtime/metrics"
	rtrace "runtime/trace"

	. "github.com/candid82/joker/core"
)

// Install interns the namespace rt's functions.
func Install() {
	ns := GLOBAL_ENV.EnsureSymbolIsNamespace(MakeSymbol("rt"))
	def := func(name string, fn func(args []Object) Object) {
		ns.Intern(MakeSymbol(name)).Value = &Proc{Fn: fn, Name: "rt/" + name}
	}
	def("gc", func(args []Object) Object {
		CheckArity(args, 0, 0)
		runtime.GC()
		return NIL
	})
	var trace bytes.Buffer
	def("trace-start", func(args []Object) Object {
		CheckArity(args, 0, 0)
		trace.Reset()
		if err := rtrace.Start(&trace); err != nil {
			panic(RT.NewError("rt/trace-start: " + err.Error()))
		}
		return NIL
	})
	def("trace-stop", func(args []Object) Object {
		CheckArity(args, 0, 0)
		rtrace.Stop()
		return MakeString(trace.String())
	})
	def("mem", func(args []Object) Object {
		CheckArity(args, 0, 0)
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		s := []metrics.Sample{{Name: "/sched/pauses/total/gc:seconds"}}
		metrics.Read(s)
		var p50, p99, max float64
		if s[0].Value.Kind() == metrics.KindFloat64Histogram {
			p50, p99, max = quantiles(s[0].Value.Float64Histogram())
		}
		mb := func(n uint64) Object { return MakeDouble(math.Round(float64(n)/(1<<20)*10) / 10) }
		us := func(sec float64) Object { return MakeDouble(math.Round(sec*1e7) / 10) }
		return NewHashMap(
			MakeKeyword("heap-mb"), mb(m.HeapAlloc),
			MakeKeyword("heap-sys-mb"), mb(m.HeapSys),
			MakeKeyword("sys-mb"), mb(m.Sys),
			MakeKeyword("gcs"), MakeInt(int(m.NumGC)),
			MakeKeyword("pause-total-ms"), MakeDouble(math.Round(float64(m.PauseTotalNs)/1e5)/10),
			MakeKeyword("pause-p50-us"), us(p50),
			MakeKeyword("pause-p99-us"), us(p99),
			MakeKeyword("pause-max-us"), us(max),
			MakeKeyword("cpus"), MakeInt(runtime.NumCPU()),
			MakeKeyword("gc-cpu-pct"), MakeDouble(math.Round(m.GCCPUFraction*1000)/10))
	})
}

// quantiles are a histogram's 50th and 99th percentiles and its greatest
// value, each its bucket's upper bound (the lower where that is infinite).
func quantiles(h *metrics.Float64Histogram) (p50, p99, max float64) {
	var total uint64
	for _, c := range h.Counts {
		total += c
	}
	if total == 0 {
		return 0, 0, 0
	}
	bound := func(i int) float64 {
		if b := h.Buckets[i+1]; !math.IsInf(b, 1) {
			return b
		}
		return h.Buckets[i]
	}
	var seen uint64
	for i, c := range h.Counts {
		if c == 0 {
			continue
		}
		seen += c
		if p50 == 0 && seen*2 >= total {
			p50 = bound(i)
		}
		if p99 == 0 && seen*100 >= total*99 {
			p99 = bound(i)
		}
		max = bound(i)
	}
	return p50, p99, max
}
