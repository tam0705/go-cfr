package deepcfr

import (
	"testing"
)

func TestAddSample(t *testing.T) {
	buf := NewReservoirBuffer(5)

	// Until buffer is full all samples should be added.
	for i := 1; i <= 5; i++ {
		buf.AddSample(Sample{Iter: i})
		samples := buf.GetSamples()
		if len(samples) != i {
			t.Errorf("expected %d samples, got %d", i, len(samples))
		}
	}

	// Once buffer is at capacity it should no longer grow,
	// but may replace existing samples.
	for i := 6; i <= 10; i++ {
		buf.AddSample(Sample{Iter: i})
		samples := buf.GetSamples()
		if len(samples) != 5 {
			t.Errorf("expected %d samples, got %d", 5, len(samples))
		}

		if buf.N != i {
			t.Errorf("expected N=%d, got %d", i, buf.N)
		}
	}

	kept := make([]int, 0, buf.Cap())
	for _, sample := range buf.GetSamples() {
		kept = append(kept, sample.Iter)
	}

	t.Logf("Final samples: %v", kept)
}

// BenchmarkAddSample_Full-24                      	30000000	        33.9 ns/op
func BenchmarkAddSample_Full(b *testing.B) {
	// Small buffer so that we test the general case where the buffer
	// will be full.
	buf := NewReservoirBuffer(1)
	for i := 0; i < b.N; i++ {
		buf.AddSample(Sample{Iter: i})
	}
}

// BenchmarkThreadSafeAddSample_Full-24            	30000000	        51.0 ns/op
func BenchmarkThreadSafeAddSample_Full(b *testing.B) {
	// Small buffer so that we test the general case where the buffer
	// will be full.
	buf := NewThreadSafeReservoirBuffer(1)
	for i := 0; i < b.N; i++ {
		buf.AddSample(Sample{Iter: i})
	}
}

// BenchmarkThreadSafeAddSample_FullParallel-24    	 5000000	       342 ns/op
func BenchmarkThreadSafeAddSample_FullParallel(b *testing.B) {
	// Small buffer so that we test the general case where the buffer
	// will be full.
	buf := NewThreadSafeReservoirBuffer(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf.AddSample(Sample{Iter: 1})
		}
	})
}