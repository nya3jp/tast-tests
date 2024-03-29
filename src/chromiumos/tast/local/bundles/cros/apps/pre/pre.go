// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains preconditions for apps tests
package pre

import "chromiumos/tast/testing/hwdep"

var stableModels = []string{
	// Top VK usage board in 2020 -- convertible, ARM.
	"hana",
	// Another top board -- convertible, x64.
	"snappy",
	// Kukui family, not much usage, but very small tablet.
	"kodama",
	"krane",
	// Convertible chromebook, top usage in 2018 and 2019.
	"cyan",
	// Random boards on the top boards for VK list.
	"bobba360",
	"bobba",
	"kefka",
	"coral",
	"betty",
}

// AppsStableModels is a shortlist of models aiming to run critical inputs tests.
// More information refers to http://b/161415599.
var AppsStableModels = hwdep.Model(stableModels...)

// AppsUnstableModels is a list of models to run inputs tests at 'informational' so that we know once they are stable enough to be promoted to CQ.
// kevin64 is an experimental board does not support nacl, which fails Canvas installation.
// To stabilize the tests, have to exclude entire kevin model as no distinguish between kevin and kevin64.
var AppsUnstableModels = hwdep.SkipOnModel(append(stableModels, "kevin")...)
