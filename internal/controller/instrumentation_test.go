// SPDX-License-Identifier: AGPL-3.0-only

package controller

import (
	"testing"

	"go.miloapis.com/instrumentation/instrumenttest"
)

func TestMetricsAreExported(t *testing.T) {
	instrumenttest.AssertNothingStranded(t)
}

func TestStandardLogger(t *testing.T) {
	instrumenttest.AssertStandardLogger(t)
}

func TestMetricsLint(t *testing.T) {
	instrumenttest.Lint(t)
}
