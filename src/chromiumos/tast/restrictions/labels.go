// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package restrictions manages restrictions on in test bundles, including
// both local and remote bundles of the same name.
package restrictions

const (
	// FragileUIMatcherLabel is the label of a Test and Fixture that declares
	// the entity uses fragile UI node matcher using the a11y tree node name.
	FragileUIMatcherLabel = "use_fragile_ui_matcher"

	// UseFragileUIMatcherForExternalDeps is the label for Tests and Fixtures
	// that need to use fragile UI matcher but in mainline test.
	UseFragileUIMatcherForExternalDeps = "fragile_ui:external_deps"

	// UseFragileUIMatcherForMigrating is the label for Tests and Fixtures
	// that uses fragile UI matcher because the test was written before the
	// FragileUIMatcherLabel is introduced.
	// Such test should be rewritten not to depend on the fragile UI matcher,
	// or changed to another label that better describes the reason.
	UseFragileUIMatcherForMigrating = "fragile_ui:migrating"
)

// ExceptionReasonLabels is list of labels that work as the reason for having
// FragileUIMatcherLabel in a mainline test.
var ExceptionReasonLabels = []string{
	UseFragileUIMatcherForExternalDeps,
	UseFragileUIMatcherForMigrating,
}
