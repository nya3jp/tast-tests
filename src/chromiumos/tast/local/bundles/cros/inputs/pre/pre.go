// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains preconditions for policy tests
package pre

// ExcludeModels is list of models are not supposed to run inputs tests.
var ExcludeModels = []string{
	// Platform kevin64. It's the experimental board to verify arm64 user land.
	"kevin64",
}
