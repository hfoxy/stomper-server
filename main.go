package stomperserver

import (
	"context"
	"flag"
	"github.com/hfoxy/stomper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
)

var version = "v0.1.5"
var sugar *zap.SugaredLogger

var memoryLimit = flag.Int("memory-limit", getEnvInt("MEMORY_LIMIT", 32212254720), "memory limit (defaults to 30MiB)")
var addr = flag.String("addr", getEnvString("BIND_ADDRESS", ":8448"), "http service address")
var compression = flag.String("compression", getEnvString("COMPRESSION", "true"), "enable compression")
var dataSource = flag.String("data-source", getEnvString("DATA_SOURCE", "redis"), "data source (only supports 'redis' currently)")

func healthHandler(writer http.ResponseWriter, _ *http.Request) {
	_, err := writer.Write([]byte("ok"))
	if err != nil {
		return
	}
}

func DefaultSetup(presetup func(*stomper.Server) error) *stomper.Server {
	flag.Parse()
	log.SetFlags(0)

	sugar = logInit()
	if dataSource == nil || (*dataSource != "redis" && *dataSource != "none") {
		sugar.Errorf("unknown data source: %s", *dataSource)
		os.Exit(1)
		return nil
	}

	debug.SetMemoryLimit(int64(*memoryLimit))

	comp := *compression
	stompServer := &stomper.Server{
		Sugar:       sugar,
		Compression: comp == "true",
	}

	ctx := context.TODO()
	if *dataSource == "redis" {
		setupRedis(ctx, stompServer)
	}

	if presetup != nil {
		err := presetup(stompServer)
		if err != nil {
			sugar.Errorf("unable to run pre-setup: %v", err)
			return nil
		}
	}

	stompServer.Setup()
	stompServer.Sugar.Infof("staring stomper %s...", version)

	http.HandleFunc("/wss/websocket", stompServer.WssHandler)
	http.HandleFunc("/health", healthHandler)
	log.Fatal(http.ListenAndServe(*addr, nil))
	return stompServer
}

func logInit() *zap.SugaredLogger {
	pe := zap.NewProductionEncoderConfig()

	pe.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(pe)

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zap.InfoLevel),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stderr), zap.WarnLevel),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stderr), zap.ErrorLevel),
	)

	l := zap.New(core)

	return l.Sugar()
}

func getEnvString(variable string, def string) string {
	if val, ok := os.LookupEnv(variable); ok {
		return val
	} else {
		return def
	}
}

func getEnvInt(variable string, def int) int {
	if val, ok := os.LookupEnv(variable); ok {
		result, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			panic(err)
		}

		return int(result)
	} else {
		return def
	}
}
