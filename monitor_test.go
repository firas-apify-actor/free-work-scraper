package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMonitorCapDropsOldestAndPersists(t *testing.T) {
	c := &Client{dir: t.TempDir()}
	m, err := LoadMonitor(c, "task1")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= maxSeen+5; i++ {
		m.Add(i, "t")
	}
	m.Add(maxSeen+5, "later") // re-adding keeps the first-seen time
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	m2, err := LoadMonitor(c, "task1")
	if err != nil || len(m2.Seen) != maxSeen {
		t.Fatalf("len=%d err=%v", len(m2.Seen), err)
	}
	if _, ok := m2.FirstSeen(5); ok {
		t.Fatal("oldest ids must be dropped")
	}
	if at, ok := m2.FirstSeen(maxSeen + 5); !ok || at != "t" {
		t.Fatalf("at=%q ok=%v", at, ok)
	}
	if _, err := LoadMonitor(c, "../evil"); err == nil {
		t.Fatal("bad stateKey must be rejected")
	}
}

func TestOnlyNewSkipsSeen(t *testing.T) {
	now := time.Now()
	body := "[" + job(1, "full", "fr~x", 600, now) + "," + job(2, "full", "fr~x", 600, now) + "]"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
	defer srv.Close()
	var got []Mission
	st, err := testFetcher(srv.URL).Search(context.Background(), defaultInput(),
		func(id int) bool { return id == 1 }, func(m Mission) bool { got = append(got, m); return true })
	if err != nil || len(got) != 1 || got[0].ID != 2 || st.Seen != 1 || st.Returned != 1 {
		t.Fatalf("got=%v st=%+v err=%v", got, st, err)
	}
}

func TestSlackTextAtMost20Lines(t *testing.T) {
	var ms []Mission
	for i := 0; i < 50; i++ {
		ms = append(ms, Mission{Title: "A & <B>", URL: "https://x/" + string(rune('a'+i%26)), RemoteMode: "full", Location: "Paris"})
	}
	txt := slackText(ms)
	if n := strings.Count(txt, "\n") + 1; n != slackMaxLine {
		t.Fatalf("lines=%d", n)
	}
	if !strings.Contains(txt, "…and 32 more") || strings.Contains(txt, "<B>") {
		t.Fatal(txt)
	}
	if n := strings.Count(slackText(ms[:3]), "\n") + 1; n != 4 {
		t.Fatalf("small message lines=%d", n)
	}
}

func TestSendSlackOneRequestAndNoSecretInErrors(t *testing.T) {
	var calls int
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		got = string(b)
	}))
	if err := sendSlack(context.Background(), srv.URL+"/secret-token", []Mission{{Title: "T", URL: "u"}}); err != nil || calls != 1 || !strings.Contains(got, `"text"`) {
		t.Fatalf("err=%v calls=%d body=%s", err, calls, got)
	}
	srv.Close()
	err := sendSlack(context.Background(), srv.URL+"/secret-token", []Mission{{Title: "T"}})
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error must exist and not leak the webhook: %v", err)
	}
}
