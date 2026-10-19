package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	z    zerolog.Logger
	file *lumberjack.Logger
}

func New(level string, fileCfg *FileConfig) *Logger {
	var writers []io.Writer

	writers = append(writers, zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	})

	var file *lumberjack.Logger
	if fileCfg != nil && fileCfg.Enabled {
		file = &lumberjack.Logger{
			Filename:   fileCfg.Path,
			MaxSize:    fileCfg.MaxSizeMB,
			MaxBackups: fileCfg.MaxFiles,
			MaxAge:     30,
			Compress:   true,
		}
		writers = append(writers, file)
	}

	multi := io.MultiWriter(writers...)

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	z := zerolog.New(multi).Level(lvl).With().Timestamp().Logger()
	return &Logger{z: z, file: file}
}

func (l *Logger) Close() error {
	if l.file == nil {
		return nil
	}
	return l.file.Close()
}

type FileConfig struct {
	Enabled   bool
	Path      string
	MaxSizeMB int
	MaxFiles  int
}

func (l *Logger) With() zerolog.Context {
	return l.z.With()
}

func (l *Logger) Debug() *zerolog.Event {
	return l.z.Debug()
}

func (l *Logger) Info() *zerolog.Event {
	return l.z.Info()
}

func (l *Logger) Warn() *zerolog.Event {
	return l.z.Warn()
}

func (l *Logger) Error() *zerolog.Event {
	return l.z.Error()
}

func (l *Logger) Fatal() *zerolog.Event {
	return l.z.Fatal()
}

func (l *Logger) Sub(name string) *Logger {
	return &Logger{z: l.z.With().Str("component", name).Logger()}
}
