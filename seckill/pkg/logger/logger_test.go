package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit_DebugLevel(t *testing.T) {
	Init("debug", "json")
	assert.NotNil(t, Log)
}

func TestInit_InfoLevel(t *testing.T) {
	Init("info", "json")
	assert.NotNil(t, Log)
}

func TestInit_ConsoleFormat(t *testing.T) {
	Init("debug", "console")
	assert.NotNil(t, Log)
}

func TestSync_DoesNotPanic(t *testing.T) {
	Init("debug", "json")
	assert.NotPanics(t, func() { Sync() })
}
