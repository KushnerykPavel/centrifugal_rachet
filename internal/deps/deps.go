// Package deps ensures all direct dependencies are tracked in go.mod.
// This file exists because go mod tidy removes unused require directives,
// but downstream plans depend on these packages being present in go.sum.
package deps

import (
	_ "github.com/KushnerykPavel/go-doubleratchet"
	_ "github.com/centrifugal/centrifuge-go"
	_ "github.com/prometheus/client_golang/prometheus"
	_ "github.com/stretchr/testify/assert"
)
