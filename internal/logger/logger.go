package logger

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func InitLogger(logLevel, logFormat string) {
	// Check if JSON logging is required
	if logFormat == "json" {
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		// Set up colored console writer
		output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}

		output.FormatLevel = func(i interface{}) string {
			if ll, ok := i.(string); ok {
				switch ll {
				case "trace":
					return fmt.Sprintf("\033[96m%s\033[0m", ll) // Cyan
				case "debug":
					return fmt.Sprintf("\033[36m%s\033[0m", ll) // Cyan
				case "info":
					return fmt.Sprintf("\033[32m%s\033[0m", ll) // Green
				case "warn":
					return fmt.Sprintf("\033[33m%s\033[0m", ll) // Yellow
				case "error":
					return fmt.Sprintf("\033[31m%s\033[0m", ll) // Red
				case "fatal":
					return fmt.Sprintf("\033[35m%s\033[0m", ll) // Purple
				case "panic":
					return fmt.Sprintf("\033[45m%s\033[0m", ll) // Bold Purple
				default:
					return ll
				}
			}
			return i.(string)
		}

		log.Logger = log.Output(output)
	}

	// Set the global log level based on the provided log level
	switch logLevel {
	case "trace":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
