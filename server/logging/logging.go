package logging

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogging(level zap.AtomicLevel, outputFiles, errorFiles []string) {
	encoderConfig := zap.NewProductionEncoderConfig()

	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("[2006-01-02 15:04:05] -")
	encoderConfig.ConsoleSeparator = " "
	encoderConfig.EncodeLevel = func(level zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(fmt.Sprintf("%s:", level.CapitalString()))
	}
	encoderConfig.EncodeCaller = nil

	config := zap.NewProductionConfig()
	config.EncoderConfig = encoderConfig
	config.Encoding = "console"
	config.Level = level
	config.OutputPaths = outputFiles
	config.ErrorOutputPaths = errorFiles

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	zap.ReplaceGlobals(logger)
}
