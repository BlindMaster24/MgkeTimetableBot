package api

import (
	"net/http"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/gin-gonic/gin"
)

type Server struct {
	engine *gin.Engine
	cache  *cache.RaspCache
	port   int
}

func NewServer(cache *cache.RaspCache, port int) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &Server{
		engine: engine,
		cache:  cache,
		port:   port,
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.engine.GET("/api/info", s.handleInfo)
	s.engine.GET("/api/groups", s.handleGroups)
	s.engine.GET("/api/teachers", s.handleTeachers)
	s.engine.GET("/api/group/:name", s.handleGroupByName)
	s.engine.GET("/api/teacher/:name", s.handleTeacherByName)
	s.engine.GET("/api/parser-health", s.handleParserHealth)
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
