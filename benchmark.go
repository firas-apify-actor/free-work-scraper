package main

import (
	"math"
	"sort"
)

const (
	minBenchmarkSample = 5  // never charge for (or publish) a benchmark on fewer rated missions
	maxBenchmarks      = 10 // ponytail: caps the bill of one run; raise if users ask for more groups
)

type Benchmark struct {
	Role   string  `json:"role"`
	Region string  `json:"region"` // "France" = all regions
	Sample int     `json:"sampleSize"`
	Median float64 `json:"median"`
	P25    float64 `json:"p25"`
	P75    float64 `json:"p75"`
}

func percentile(sorted []float64, p float64) float64 {
	pos := p * float64(len(sorted)-1)
	lo, hi := int(math.Floor(pos)), int(math.Ceil(pos))
	return sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
}

// computeBenchmarks groups rated missions by role (all of France) and by role+region,
// using the midpoint of each mission's day-rate range. Groups under the minimum sample are dropped.
func computeBenchmarks(ms []Mission) []Benchmark {
	groups := map[[2]string][]float64{}
	for _, m := range ms {
		if m.role == "" || (m.DailyRateMin == nil && m.DailyRateMax == nil) {
			continue
		}
		lo, hi := m.DailyRateMin, m.DailyRateMax
		if lo == nil {
			lo = hi
		}
		if hi == nil {
			hi = lo
		}
		mid := (*lo + *hi) / 2
		groups[[2]string{m.role, "France"}] = append(groups[[2]string{m.role, "France"}], mid)
		if m.Region != "" {
			groups[[2]string{m.role, m.Region}] = append(groups[[2]string{m.role, m.Region}], mid)
		}
	}
	var out []Benchmark
	for k, v := range groups {
		if len(v) < minBenchmarkSample {
			continue
		}
		sort.Float64s(v)
		out = append(out, Benchmark{k[0], k[1], len(v), percentile(v, .5), percentile(v, .25), percentile(v, .75)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sample != out[j].Sample {
			return out[i].Sample > out[j].Sample
		}
		return out[i].Role+out[i].Region < out[j].Role+out[j].Region
	})
	if len(out) > maxBenchmarks {
		out = out[:maxBenchmarks]
	}
	return out
}
