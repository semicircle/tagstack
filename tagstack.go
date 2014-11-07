package tagstack

import (
	"log"
	"os"
)

var (
	// Normal logger.
	Logger = log.New(os.Stdout, "[tagstack]", log.LstdFlags)
	// Debug logger.
	DebugLogger = log.New(os.Stdout, "[tagstack.debug]", log.LstdFlags)
)
