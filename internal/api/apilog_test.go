package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// failingEngine mirrors the production middleware order: recovery outside,
// observation inside, so a panic is logged by observe and answered by gin.
func failingEngine(s *Server) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(s.observe())
	return engine
}

func TestAFailingRequestIsLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	s := &Server{logger: &logger}
	engine := failingEngine(s)
	engine.GET("/boom", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	line := buf.String()
	if !strings.Contains(line, "api request failed") || !strings.Contains(line, "/boom") || !strings.Contains(line, "500") {
		t.Fatalf("log = %q, want the failed request line", line)
	}
}

func TestAPanicIsLoggedAndStillAnswered(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	s := &Server{logger: &logger}
	engine := failingEngine(s)
	engine.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(buf.String(), "api request failed") {
		t.Fatalf("log = %q, want the panic line", buf.String())
	}
}

func TestASilentServerKeepsWorking(t *testing.T) {
	s := &Server{}
	engine := failingEngine(s)
	engine.GET("/boom", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
