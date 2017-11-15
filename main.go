package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap/zapcore"

	"go.uber.org/zap"

	"github.com/alastairruhm/influxdb-gateway/gateway"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var (
	configFilePath string
	logFilePath    string
	logger         *zap.Logger
)

func init() {
	flag.StringVar(&configFilePath, "config-file-path", "/etc/influxdb-gateway.toml", "config file path")
	flag.StringVar(&logFilePath, "log-file-path", "/var/log/influxdb-gateway.log", "log file path")
	flag.Parse()

	// zap log config
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel
	})

	consoleDebugging := zapcore.Lock(os.Stdout)
	consoleErrors := zapcore.Lock(os.Stderr)
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())

	jsonEncoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())

	// ref: https://github.com/uber-go/zap/blob/master/FAQ.md#does-zap-support-log-rotation
	// lumberjack.Logger is already safe for concurrent use, so we don't need to
	// lock it.
	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    100,
		MaxBackups: 5,
		MaxAge:     7,
	})

	logger = zap.New(
		zapcore.NewTee(
			zapcore.NewCore(consoleEncoder, consoleErrors, highPriority),
			zapcore.NewCore(consoleEncoder, consoleDebugging, lowPriority),
			zapcore.NewCore(jsonEncoder, w, highPriority),
		),
	)
}

func main() {
	c, err := gateway.LoadConfig(configFilePath)
	if err != nil {
		logger.Fatal(err.Error())
	}
	gateway, err := gateway.New(c, logger)
	if err != nil {
		logger.Fatal(err.Error())
	}

	err = gateway.Open()
	if err != nil {
		logger.Fatal(err.Error())
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)
	logger.Info("Listening for signals")
	select {
	case <-signalCh:
		gateway.Close()
		logger.Info("Signal received, shutdown...")
	}
}
