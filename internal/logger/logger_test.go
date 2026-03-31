package logger

import (
	"testing"

	"go.uber.org/zap"
)

func TestInitLogger(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		format    string
		wantError bool
	}{
		{
			name:      "valid json format with info level",
			level:     "info",
			format:    "json",
			wantError: false,
		},
		{
			name:      "valid development format with debug level",
			level:     "debug",
			format:    "development",
			wantError: false,
		},
		{
			name:      "valid json format with error level",
			level:     "error",
			format:    "json",
			wantError: false,
		},
		{
			name:      "invalid log level",
			level:     "invalid",
			format:    "json",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := InitLogger(tt.level, tt.format)
			if (err != nil) != tt.wantError {
				t.Errorf("InitLogger() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && logger == nil {
				t.Errorf("InitLogger() returned nil logger")
			}
			if logger != nil {
				defer logger.Sync()
			}
		})
	}
}

func TestInitLoggerWithSampling(t *testing.T) {
	tests := []struct {
		name             string
		level            string
		format           string
		sampleInitial    int
		sampleThereafter int
		wantError        bool
	}{
		{
			name:             "valid json format with sampling",
			level:            "info",
			format:           "json",
			sampleInitial:    100,
			sampleThereafter: 100,
			wantError:        false,
		},
		{
			name:             "valid development format with sampling",
			level:            "debug",
			format:           "development",
			sampleInitial:    50,
			sampleThereafter: 50,
			wantError:        false,
		},
		{
			name:             "invalid log level",
			level:            "invalid",
			format:           "json",
			sampleInitial:    100,
			sampleThereafter: 100,
			wantError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := InitLoggerWithSampling(tt.level, tt.format, tt.sampleInitial, tt.sampleThereafter)
			if (err != nil) != tt.wantError {
				t.Errorf("InitLoggerWithSampling() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && logger == nil {
				t.Errorf("InitLoggerWithSampling() returned nil logger")
			}
			if logger != nil {
				defer logger.Sync()
			}
		})
	}
}

func TestLoggerOutput(t *testing.T) {
	logger, err := InitLogger("info", "json")
	if err != nil {
		t.Fatalf("InitLogger() error = %v", err)
	}
	defer logger.Sync()

	// Test that logger can output different levels
	logger.Debug("debug message")
	logger.Info("info message", zap.String("key", "value"))
	logger.Warn("warn message", zap.Int("count", 42))
	logger.Error("error message", zap.Error(nil))
}

func TestLoggerLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			logger, err := InitLogger(level, "json")
			if err != nil {
				t.Fatalf("InitLogger() error = %v", err)
			}
			defer logger.Sync()

			// Verify logger was created successfully
			if logger == nil {
				t.Errorf("InitLogger() returned nil logger for level %s", level)
			}
		})
	}
}

func TestLoggerFormats(t *testing.T) {
	formats := []string{"json", "development"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			logger, err := InitLogger("info", format)
			if err != nil {
				t.Fatalf("InitLogger() error = %v", err)
			}
			defer logger.Sync()

			// Verify logger was created successfully
			if logger == nil {
				t.Errorf("InitLogger() returned nil logger for format %s", format)
			}
		})
	}
}
