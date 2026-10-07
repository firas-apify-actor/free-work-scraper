package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testFetcher(url string) *Fetcher {
	return &Fetcher{base: url, client: http.DefaultClient, retryBase: time.Millisecond}
}

func job(id int, remote, key string, maxRate int, published time.Time) string {
	return fmt.Sprintf(`{"id":%d,"title":"t%d","slug":"s%d","publishedAt":%q,"remoteMode":%q,"maxDailySalary":%d,
"contracts":["contractor"],"job":{"slug":"j"},"company":{"name":"c"},"location":{"label":"X","key":%q}}`,
		id, id, id, published.Format(time.RFC3339), remote, maxRate, key)
}

func TestRetryHonoursRetryAfter(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	var v []rawJob
	if err := testFetcher(srv.URL).getJSON(context.Background(), "/x", &v); err != nil || n != 2 {
		t.Fatalf("err=%v calls=%d", err, n)
	}
}

func TestSearchFiltersAndDedupes(t *testing.T) {
	now := time.Now()
	body := "[" + strings.Join([]string{
		job(1, "full", "fr~ile-de-france~paris~paris", 600, now),
		job(1, "full", "fr~ile-de-france~paris~paris", 600, now),                    // dupe
		job(2, "none", "fr~ile-de-france~paris~paris", 600, now),                    // wrong remote
		job(3, "full", "fr~occitanie~haute-garonne~toulouse", 600, now),             // wrong location
		job(4, "full", "fr~ile-de-france~paris~paris", 300, now),                    // below min rate
		job(5, "full", "fr~ile-de-france~paris~paris", 600, now.AddDate(0, 0, -30)), // too old
	}, ",") + "]"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
	defer srv.Close()
	in := defaultInput()
	in.RemoteMode, in.Locations, in.MinDailyRate = "full", []string{"Île-de-France"}, 500
	var got []Mission
	st, err := testFetcher(srv.URL).Search(context.Background(), in, nil, func(m Mission) bool { got = append(got, m); return true })
	if err != nil || len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if st.Scanned != 6 || st.Dupes != 1 || st.Filtered != 4 || st.Returned != 1 {
		t.Fatalf("stats=%+v", st)
	}
}

func TestSearchFailsLoudly(t *testing.T) {
	for name, body := range map[string]string{
		"empty unfiltered": `[]`,
		"missing fields":   `[{"id":1}]`,
		"wrong shape":      `{"error":"x"}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		in := defaultInput()
		in.Keywords = ""
		if _, err := testFetcher(srv.URL).Search(context.Background(), in, nil, func(Mission) bool { return true }); err == nil {
			t.Errorf("%s: expected error", name)
		}
		srv.Close()
	}
}

func TestPlainTextRedactsPersonalData(t *testing.T) {
	got := plainText("<p>Contact: jean.dupont@acme.fr ou 06 12 34 56 78</p><ul><li>Go &amp; K8s</li></ul>")
	if strings.Contains(got, "@") || strings.Contains(got, "06 12") || !strings.Contains(got, "Go & K8s") {
		t.Fatal(got)
	}
}
