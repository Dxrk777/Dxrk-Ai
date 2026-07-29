// Package di provides dependency injection utilities using uber-go/fx.
package di

import (
	"go.uber.org/fx"
)

// Module re-exports an fx module for composition.
func Module(name string, opts ...fx.Option) fx.Option {
	return fx.Module(name, opts...)
}
