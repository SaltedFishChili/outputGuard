package log

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	once        sync.Once
	Logger      *zap.Logger
	processName string
	logPath     string
)

func init() {
	switch runtime.GOOS {
	case "windows":
		logPath = "D:/logs"
	case "linux":
		logPath = "/data/logs"
	case "darwin":
		logPath = "/tmp/logs"
	}
	executablePath, err := os.Executable()
	if err != nil {
		processName = "log.log"
		fmt.Println("获取进程名称失败，错误信息:", err.Error())
	}
	processName = filepath.Base(executablePath)
	pathName := strings.Split(processName, ".")[0]
	InitLogger(fmt.Sprintf("%s/%s-log/%s.log", logPath, pathName, processName), "DEBUG")
}

func InitLogger(filepath string, logLevel string) {
	once.Do(func() {
		atomicLevel := zap.NewAtomicLevel()
		switch logLevel {
		case "DEBUG":
			atomicLevel.SetLevel(zapcore.DebugLevel)
		case "INFO":
			atomicLevel.SetLevel(zapcore.InfoLevel)
		case "WARN":
			atomicLevel.SetLevel(zapcore.WarnLevel)
		case "ERROR":
			atomicLevel.SetLevel(zapcore.ErrorLevel)
		case "DPANIC":
			atomicLevel.SetLevel(zapcore.DPanicLevel)
		case "PANIC":
			atomicLevel.SetLevel(zapcore.PanicLevel)
		case "FATAL":
			atomicLevel.SetLevel(zapcore.FatalLevel)
		}

		encoderConfig := zapcore.EncoderConfig{
			TimeKey:       "time",
			LevelKey:      "level",
			NameKey:       "name",
			CallerKey:     "line",
			MessageKey:    "msg",
			FunctionKey:   "func",
			StacktraceKey: "stacktrace",
			LineEnding:    zapcore.DefaultLineEnding,
			EncodeLevel:   zapcore.LowercaseLevelEncoder,
			EncodeTime: func(t time.Time, pae zapcore.PrimitiveArrayEncoder) {
				pae.AppendByteString([]byte(t.Local().Format("2006-01-02 15:04:05.000")))
			},
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
			EncodeName:     zapcore.FullNameEncoder,
		}
		// 日志轮转
		writer := &lumberjack.Logger{
			// 日志名称
			Filename: filepath,
			// 日志大小限制，单位MB
			MaxSize: 1024,
			// 历史日志文件保留天数
			MaxAge: 7,
			// 最大保留历史日志数量
			MaxBackups: 10,
			// 本地时区
			LocalTime: true,
			// 历史日志文件压缩标识
			Compress: true,
		}
		defer writer.Close()
		zapCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(writer)),
			atomicLevel,
		)
		Logger = zap.New(zapCore, zap.AddCaller())
		zap.ReplaceGlobals(Logger)
	})
}
