package parser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

func TestFetchDocumentReadsThePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("the request must identify the parser")
		}
		_, _ = w.Write([]byte(`<html><body><table class="rasp"><tr><td>Математика</td></tr></table></body></html>`))
	}))
	defer server.Close()

	doc, err := FetchDocument(server.Client(), server.URL)
	if err != nil {
		t.Fatalf("FetchDocument: %v", err)
	}
	if text := doc.Text(); !strings.Contains(text, "Математика") {
		t.Fatalf("document text = %q, want the table", text)
	}
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer failing.Close()
	if _, err := FetchDocument(failing.Client(), failing.URL); err == nil {
		t.Fatal("a 500 must be an error")
	}
}

func TestFailedReportCarriesTheWarning(t *testing.T) {
	report := failedReport("groups", "http://example.test", context.DeadlineExceeded)
	if report.Source != "groups" || report.URL != "http://example.test" {
		t.Fatalf("report = %+v, want the source and the url", report)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "fetch failed") {
		t.Fatalf("warnings = %v, want one fetch failure", report.Warnings)
	}
}

func TestRunLoopStopsWithTheContext(t *testing.T) {
	var runs atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())

	loop := LoopConfig{Enabled: true, Default: 10 * time.Millisecond, ErrorDelay: 10 * time.Millisecond}
	go func() {
		time.Sleep(60 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	go func() {
		RunLoop(ctx, loop, logger.New("error", nil), func() error {
			runs.Add(1)
			return nil
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunLoop did not stop after the context was cancelled")
	}
	if runs.Load() == 0 {
		t.Fatal("RunLoop must call the parse function at least once")
	}

	disabled := LoopConfig{Enabled: false}
	RunLoop(context.Background(), disabled, logger.New("error", nil), func() error {
		t.Fatal("a disabled loop must not parse")
		return nil
	})
}
