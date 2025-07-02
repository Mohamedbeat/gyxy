package logger

import (
	"github.com/fatih/color"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func InitLogger() (*zap.Logger, error) {
	// Custom encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    colorLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create console encoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Create core with stdout output
	core := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(color.Output),
		zap.NewAtomicLevelAt(zapcore.InfoLevel),
	)

	// Create logger with options
	Logger = zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	zap.ReplaceGlobals(Logger)
	return Logger, nil
}

// Custom level encoder with colors
func colorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch l {
	case zapcore.DebugLevel:
		enc.AppendString(color.BlueString("DEBUG"))
	case zapcore.InfoLevel:
		enc.AppendString(color.GreenString("INFO"))
	case zapcore.WarnLevel:
		enc.AppendString(color.YellowString("WARN"))
	case zapcore.ErrorLevel:
		enc.AppendString(color.RedString("ERROR"))
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		enc.AppendString(color.MagentaString("CRITICAL"))
	default:
		enc.AppendString(color.WhiteString(l.CapitalString()))
	}
}

// // RequestLogger middleware with colored output
// func RequestLogger(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		start := time.Now()
//
// 		// Log the request start
// 		Logger.Info(color.GreenString("→ Incoming") + " " +
// 			color.CyanString("%-7s", r.Method) + " " +
// 			color.WhiteString(r.URL.Path) + " " +
// 			color.MagentaString("from %s", r.RemoteAddr))
//
// 		// Wrap the response writer
// 		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
//
// 		// Call the next handler
// 		next.ServeHTTP(ww, r)
//
// 		// Determine status color
// 		statusColor := color.New(color.FgGreen)
// 		if ww.Status() >= 400 && ww.Status() < 500 {
// 			statusColor = color.New(color.FgYellow)
// 		} else if ww.Status() >= 500 {
// 			statusColor = color.New(color.FgRed)
// 		}
//
// 		// Determine duration color
// 		duration := time.Since(start)
// 		durationColor := color.New(color.FgGreen)
// 		if duration > 100*time.Millisecond {
// 			durationColor = color.New(color.FgYellow)
// 		}
// 		if duration > 500*time.Millisecond {
// 			durationColor = color.New(color.FgRed)
// 		}
//
// 		// Log the request completion
// 		Logger.Info(color.GreenString("← Completed") + " " +
// 			color.CyanString("%-7s", r.Method) + " " +
// 			color.WhiteString(r.URL.Path) + " " +
// 			statusColor.Sprintf("%3d", ww.Status()) + " " +
// 			durationColor.Sprintf("%13v", duration) + " " +
// 			color.BlueString(humanizeBytes(ww.BytesWritten())))
// 	})
// }

func humanizeBytes(b int) string {
	const unit = 1024
	if b < unit {
		return color.BlueString("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return color.BlueString("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
