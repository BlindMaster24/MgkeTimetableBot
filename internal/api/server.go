package api

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/gin-gonic/gin"
)

const (
	apiCaptureLimit = 512
	healthRoute     = "/api/health"
)

type Server struct {
	engine *gin.Engine
	cache  *cache.RaspCache
	port   int
	health *health.Tracker
	build  build.Info
}

func NewServer(cache *cache.RaspCache, port int, tracker *health.Tracker, info build.Info) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &Server{
		engine: engine,
		cache:  cache,
		port:   port,
		health: tracker,
		build:  info,
	}

	engine.Use(s.observe())
	s.routes()
	return s
}

type captureWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
	keep bool
}

func (w *captureWriter) WriteHeader(code int) {
	w.keep = code >= 500
	w.ResponseWriter.WriteHeader(code)
}

func (w *captureWriter) Write(data []byte) (int, error) {
	if w.keep && w.body.Len() < apiCaptureLimit {
		clip := data
		if room := apiCaptureLimit - w.body.Len(); len(clip) > room {
			clip = clip[:room]
		}
		w.body.Write(clip)
	}
	return w.ResponseWriter.Write(data)
}

func (s *Server) observe() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		writer := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = writer

		defer func() {
			if value := recover(); value != nil {
				s.recordAPI(c, writer, start, http.StatusInternalServerError, fmt.Sprint(value))
				panic(value)
			}
			s.recordAPI(c, writer, start, writer.Status(), writer.body.String())
		}()

		c.Next()
	}
}

func (s *Server) recordAPI(c *gin.Context, writer *captureWriter, start time.Time, status int, message string) {
	if s.health == nil {
		return
	}
	path := apiPath(c)
	s.health.RecordAPI(health.APIRequest{
		Method:    c.Request.Method,
		Path:      path,
		Status:    status,
		Duration:  time.Since(start),
		Message:   message,
		SelfProbe: path == healthRoute && status == http.StatusServiceUnavailable,
	})
}

func apiPath(c *gin.Context) string {
	if route := c.FullPath(); route != "" {
		return route
	}
	return c.Request.URL.Path
}

func (s *Server) routes() {
	s.engine.GET("/api/info", s.handleInfo)
	s.engine.GET("/api/groups", s.handleGroups)
	s.engine.GET("/api/teachers", s.handleTeachers)
	s.engine.GET("/api/group/:name", s.handleGroupByName)
	s.engine.GET("/api/teacher/:name", s.handleTeacherByName)
	s.engine.GET("/api/parser-health", s.handleParserHealth)
	s.engine.GET(healthRoute, s.handleHealth)
}

func (s *Server) HandleGoogleOAuth(path string, handler http.HandlerFunc) {
	if path == "" || handler == nil {
		return
	}
	s.engine.GET(path, gin.WrapH(handler))
}

func (s *Server) Run() error {
	return s.engine.Run(":" + itoa(s.port))
}

func (s *Server) Handler() http.Handler {
	return s.engine
}

func (s *Server) handleInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name":    "MgkeTimetableBot API",
		"version": "2.0",
		"build":   s.build,
	})
}

func (s *Server) handleGroups(c *gin.Context) {
	groups := s.cache.GetGroups()
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	c.JSON(http.StatusOK, gin.H{"groups": keys})
}

func (s *Server) handleTeachers(c *gin.Context) {
	teachers := s.cache.GetTeachers()
	keys := make([]string, 0, len(teachers))
	for k := range teachers {
		keys = append(keys, k)
	}
	c.JSON(http.StatusOK, gin.H{"teachers": keys})
}

func (s *Server) handleGroupByName(c *gin.Context) {
	name := c.Param("name")
	groups := s.cache.GetGroups()
	data, ok := groups[name]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	s.cache.RecordHit()
	c.JSON(http.StatusOK, data)
}

func (s *Server) handleTeacherByName(c *gin.Context) {
	name := c.Param("name")
	teachers := s.cache.GetTeachers()
	data, ok := teachers[name]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "teacher not found"})
		return
	}
	s.cache.RecordHit()
	c.JSON(http.StatusOK, data)
}

func (s *Server) handleParserHealth(c *gin.Context) {
	stats := s.cache.Stats()
	c.JSON(http.StatusOK, stats)
}

func (s *Server) handleHealth(c *gin.Context) {
	if s.health == nil {
		c.JSON(http.StatusOK, gin.H{"status": "unknown"})
		return
	}

	snapshot := s.health.Snapshot()
	snapshot.Build = &s.build
	status := http.StatusOK
	if len(snapshot.Alerts) > 0 {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, snapshot)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
