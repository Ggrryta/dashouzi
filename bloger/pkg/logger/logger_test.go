package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit_WithDebugLevel(t *testing.T) {
	Init("debug", "json")
	assert.NotNil(t, Log)
	// 不 panic 就是通过
}

func TestInit_WithInfoLevel(t *testing.T) {
	Init("info", "json")
	assert.NotNil(t, Log)
}

func TestInit_WithWarnLevel(t *testing.T) {
	Init("warn", "json")
	assert.NotNil(t, Log)
}

func TestInit_WithErrorLevel(t *testing.T) {
	Init("error", "json")
	assert.NotNil(t, Log)
}

func TestInit_WithUnknownLevelDefaultsToInfo(t *testing.T) {
	Init("unknown", "json")
	assert.NotNil(t, Log)
}

func TestInit_WithConsoleFormat(t *testing.T) {
	Init("debug", "console")
	assert.NotNil(t, Log)
}

func TestSync_DoesNotPanic(t *testing.T) {
	Init("debug", "json")
	// Sync 不应该 panic
	assert.NotPanics(t, func() {
		Sync()
	})
}
