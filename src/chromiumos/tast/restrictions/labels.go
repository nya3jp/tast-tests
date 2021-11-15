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
)
