//go:build tools

// Package tools tracks development tool dependencies.
// This file is not compiled into the application binary.
// See: https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package tools

import (
	_ "github.com/vektra/mockery/v2"
)
