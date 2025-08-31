package utils

import (
	"runtime/debug"

	"github.com/rs/zerolog/log"
)

// SafeGo executes a function in a goroutine with panic recovery
func SafeGo(fn func()) {
	if fn == nil {
		log.Error().Msg("SafeGo called with nil function")
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Str("stack", string(debug.Stack())).
					Msg("Panic recovered in goroutine")
			}
		}()
		fn()
	}()
}

// SafeGoWithName executes a function in a goroutine with panic recovery and a name for logging
func SafeGoWithName(name string, fn func()) {
	if fn == nil {
		log.Error().Str("goroutine", name).Msg("SafeGoWithName called with nil function")
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Str("goroutine", name).
					Interface("panic", r).
					Str("stack", string(debug.Stack())).
					Msg("Panic recovered in goroutine")
			}
		}()
		fn()
	}()
}
