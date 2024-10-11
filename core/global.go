package core

import (
	"diablo/core/logging"
	"os"
)

const (
	VERBOSITY_SILENT  int = 0
	VERBOSITY_FATAL   int = 1
	VERBOSITY_ERROR   int = 2
	VERBOSITY_WARNING int = 3
	VERBOSITY_INFO    int = 4
	VERBOSITY_DEBUG   int = 5
	VERBOSITY_TRACE   int = 6
)

func SetVerbosity(verbosity int) {
	var level logging.LogLevel
	var logger logging.Logger

	if verbosity == VERBOSITY_SILENT {
		level = logging.LOG_SILENT
	} else if verbosity == VERBOSITY_FATAL {
		level = logging.LOG_FATAL
	} else if verbosity == VERBOSITY_ERROR {
		level = logging.LOG_ERROR
	} else if verbosity == VERBOSITY_WARNING {
		level = logging.LOG_WARN
	} else if verbosity == VERBOSITY_INFO {
		level = logging.LOG_INFO
	} else if verbosity == VERBOSITY_DEBUG {
		level = logging.LOG_DEBUG
	} else {
		level = logging.LOG_TRACE
	}

	logger = logging.NewPrintLogger(os.Stderr, "", level)

	logging.SetLogger(logger)
}
