//go:build tools

// Package tools keeps dependencies that are part of the planned application
// surface in go.mod before their implementation phases land.
package tools

import (
	_ "github.com/pelletier/go-toml/v2"
	_ "github.com/spf13/cobra"
)
