package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChargeLocalTestModeRespectsLimit(t *testing.T) {
	t.Setenv("ACTOR_TEST_PAY_PER_EVENT", "true")
	t.Setenv("ACTOR_MAX_TOTAL_CHARGE_USD", "3")
	c := &Client{dir: t.TempDir()}
	var got int
	var limit bool
	for i := 0; i < 5; i++ {
		n, l, err := c.Charge("mission-returned", 1)
		if err != nil {
			t.Fatal(err)
		}
		got += n
		if l {
			limit = true
			break
		}
	}
	if got != 3 || !limit {
		t.Fatalf("charged=%d limit=%v", got, limit)
	}
	if n, _, _ := c.Charge("mission-returned", 1); n != 0 {
		t.Fatal("must not charge past the limit")
	}
	files, _ := os.ReadDir(filepath.Join(c.dir, "datasets", "charging-log"))
	if len(files) != 3 || c.ChargedCounts()["mission-returned"] != 3 {
		t.Fatalf("log files=%d counts=%v", len(files), c.ChargedCounts())
	}
}

func TestChargeFreeRunIsNoop(t *testing.T) {
	t.Setenv("ACTOR_TEST_PAY_PER_EVENT", "")
	c := &Client{dir: t.TempDir()}
	if n, l, err := c.Charge("mission-returned", 1); n != 1 || l || err != nil {
		t.Fatal(n, l, err)
	}
	if _, err := os.Stat(filepath.Join(c.dir, "datasets", "charging-log")); err == nil {
		t.Fatal("free run must not write a charging log")
	}
}

func TestChargePlatformSendsIdempotencyKeyAndCountsPrice(t *testing.T) {
	var key, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("idempotency-key")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(201)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := &Client{runID: "RUN1", token: "tok", apiBase: srv.URL}
	c.charging = &charging{prices: map[string]float64{"mission-returned": 0.0025}, max: 0.005, counts: map[string]int{}}
	n, limit, err := c.Charge("mission-returned", 5) // only 2 fit in $0.005
	if err != nil || n != 2 || !limit {
		t.Fatalf("n=%d limit=%v err=%v", n, limit, err)
	}
	var b struct {
		EventName string
		Count     int
	}
	json.Unmarshal([]byte(body), &b)
	if !strings.HasPrefix(key, "RUN1-mission-returned-") || b.EventName != "mission-returned" || b.Count != 2 {
		t.Fatalf("key=%q body=%s", key, body)
	}
}

func TestBenchmarks(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	var ms []Mission
	for _, r := range []float64{400, 450, 500, 550, 600} {
		ms = append(ms, Mission{role: "devops", Region: "Île-de-France", DailyRateMin: f(r - 50), DailyRateMax: f(r + 50)})
	}
	ms = append(ms, Mission{role: "sre", Region: "Occitanie", DailyRateMin: f(500)}) // sample of 1: dropped
	ms = append(ms, Mission{role: "devops", Region: "Occitanie"})                    // no rate: ignored
	bs := computeBenchmarks(ms)
	if len(bs) != 2 { // devops/France and devops/Île-de-France
		t.Fatalf("%+v", bs)
	}
	if b := bs[0]; b.Sample != 5 || b.Median != 500 || b.P25 != 450 || b.P75 != 550 {
		t.Fatalf("%+v", b)
	}
	if len(computeBenchmarks(ms[:4])) != 0 {
		t.Fatal("sample < 5 must yield no benchmark")
	}
}
