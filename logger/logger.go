package logger

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/ewinjuman/go-lib/helper/convert"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	RotateLogs "github.com/lestrrat-go/file-rotatelogs"
)

type Logger struct {
	loggerSys *zap.Logger
	InstID    string
	Options   Options
	ThreadID  string
}
type Fields map[string]interface{}

type Interface interface {
	InfoSys(message string, fields ...zap.Field)
	InfoTdr(message string, fields ...zap.Field)
	Error(message string, fields ...zap.Field)
}

func (l *Logger) Printf(s string, v ...interface{}) {
	if len(v) == 4 {
		l.loggerSys.Info("",
			zap.String("level", "INFO"),
			zap.String("request_id", l.ThreadID),
			zap.String("query", v[3].(string)),
			zap.String("duration ", fmt.Sprintf("%.3fms", v[1].(float64))),
			zap.Int64("affected-rows", v[2].(int64)),
			zap.String("source", v[0].(string)),
		)
	} else {
		l.loggerSys.Info("",
			zap.String("level", "INFO"),
			zap.String("request_id", l.ThreadID),
			zap.Any("value", v),
		)
	}
}
func (l *Logger) Print(v ...interface{}) {
	if len(v) < 2 {
		return
	}
	switch v[0] {
	case "sql":
		delimiter := "/"
		rightOfDelimiter := strings.Join(strings.Split(v[1].(string), delimiter)[4:], delimiter)
		l.loggerSys.Info("",
			zap.String("level", "INFO"),
			zap.String("request_id", l.ThreadID),
			zap.String("query", v[3].(string)),
			zap.Any("values", v[4]),
			zap.Float64("duration", float64(v[2].(time.Duration))/float64(time.Millisecond)),
			zap.Int64("affected-rows", v[5].(int64)),
			zap.String("source", rightOfDelimiter),
		)
	default:
		delimiter := "/"
		rightOfDelimiter := strings.Join(strings.Split(v[1].(string), delimiter)[4:], delimiter)
		l.loggerSys.Info("",
			zap.String("request_id", l.ThreadID),
			zap.Any("values", v[2:]),
			zap.String("source", rightOfDelimiter),
		)
	}
}

func getRotateWriter(config Options) zapcore.WriteSyncer {
	if config.FileName == "" {
		config.FileName = "log"
	}
	rotate, err := RotateLogs.New(
		config.FileLocation+"%Y-%m-%d."+config.FileName,
		RotateLogs.WithMaxAge(config.FileMaxAge*24*time.Hour),
		RotateLogs.WithRotationTime(time.Hour),
	)
	if err != nil {
		panic(err)
	}
	return zapcore.AddSync(rotate)
}

func New(config Options) *Logger {
	var cores []zapcore.Core
	var writer zapcore.WriteSyncer

	if config.Stdout {
		writer = zapcore.AddSync(os.Stdout)
	} else {
		writer = getRotateWriter(config)
	}

	core := zapcore.NewCore(getEncoder(), writer, zapcore.InfoLevel)
	cores = append(cores, core)

	combinedCore := zapcore.NewTee(cores...)

	loggerSys := zap.New(combinedCore,
		zap.AddCallerSkip(3),
		zap.AddCaller(),
	)

	l := &Logger{
		loggerSys: loggerSys,
	}
	l.Options = config
	return l
}

func getEncoder() zapcore.Encoder {
	config := zapcore.EncoderConfig{
		TimeKey:        "time",
		MessageKey:     "message",
		EncodeDuration: MillisDurationEncoder,
		EncodeTime:     TDRLogTimeEncoder,
		LineEnding:     zapcore.DefaultLineEnding,
	}
	return zapcore.NewConsoleEncoder(config)
}

func TDRLogTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	location, _ := time.LoadLocation("Asia/Jakarta")
	enc.AppendString(t.In(location).Format("2006-01-02 15:04:05.999"))
}

func MillisDurationEncoder(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendInt64(d.Milliseconds())
}

func (l *Logger) Error(message string, fields ...zap.Field) {
	_, fn, line, _ := runtime.Caller(1)
	file := fmt.Sprintf("%s:%d - ", fn, line)
	l.loggerSys.Error(file+message, fields...)
}

func (l *Logger) InfoSys(message string, fields ...zap.Field) {
	l.loggerSys.Info(message, fields...)
}

func (l *Logger) MaskingJson(data interface{}) interface{} {
	jsonString := convert.ObjectToString(data)
	path := strings.Split(l.Options.MaskingLogJsonPath, "|")

	for _, key := range path {
		value := gjson.Get(jsonString, key)
		if value.String() != "" && value.Type.String() != "Null" {
			switch value.Type.String() {
			case "String":
				jsonString, _ = sjson.Set(jsonString, key, "******")
			case "JSON":
				jsonString, _ = sjson.Set(jsonString, key, "***Mask JSON***")
			case "False":
			case "True":
			default:
				jsonString, _ = sjson.Set(jsonString, key, 00000)
			}
		}
	}

	var mapData interface{}
	convert.StringToObject(jsonString, &mapData)
	return mapData
}

func (l *Logger) MaskingJsonWithPath(data interface{}, jsonPath string) interface{} {
	jsonString := convert.ObjectToString(data)
	path := strings.Split(jsonPath, "|")

	for _, key := range path {
		value := gjson.Get(jsonString, key)
		if value.String() != "" && value.Type.String() != "Null" {
			switch value.Type.String() {
			case "String":
				jsonString, _ = sjson.Set(jsonString, key, "******")
			case "JSON":
				jsonString, _ = sjson.Set(jsonString, key, "***Mask JSON***")
			case "False":
			case "True":
			default:
				jsonString, _ = sjson.Set(jsonString, key, 00000)
			}
		}
	}

	var mapData interface{}
	convert.StringToObject(jsonString, &mapData)
	return mapData
}
