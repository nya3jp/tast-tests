// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains preconditions for inputs tests.
package pre

import (
	"chromiumos/tast/testing/hwdep"
)

// StableModels is a list of boards that stable enough and aim to run inputs tests in CQ.
var StableModels = []string{
	"betty",
	// Random boards on the top boards for VK list.
	"bobba",
	"bobba360",
	"casta",
	"coral",
	"kefka",
	// Convertible chromebook, top usage in 2018 and 2019.
	"cyan",
	// Top VK usage board in 2020 -- convertible, ARM.
	"hana",
	// Kukui family, not much usage, but very small tablet.
	"kodama",
	"krane",
	"kukui",
	// Another top board -- convertible, x64.
	"snappy",
}

// GrammarEnabledModels is a list boards where Grammar Check is enabled.
var GrammarEnabledModels = []string{
	"betty",
	"octopus",
	"nocturne",
	"hatch",
}

// MultiwordEnabledModels is a subset of boards where multiword suggestions are
// enabled. The multiword feature is enabled on all 4gb boards, with a list of
// 2gb boards having the feature explicitly disabled. See the following link
// for a list of all boards where the feature is disabled.
// https://source.chromium.org/search?q=f:make.defaults%20%22-ondevice_text_suggestions%22&ss=chromiumos&start=31
var MultiwordEnabledModels = []string{
	"betty",
	"octopus",
	"nocturne",
	"hatch",
}

// InputsStableModels is a shortlist of models aiming to run critical inputs tests.
// More information refers to http://b/161415599.
var InputsStableModels = hwdep.Model(StableModels...)

// InputsUnstableModels is a list of models to run inputs tests at 'informational' so that we know once they are stable enough to be promoted to CQ.
// kevin64 is an experimental board does not support nacl, which fails Canvas installation.
// To stabilize the tests, have to exclude entire kevin model as no distinguish between kevin and kevin64.
var InputsUnstableModels = hwdep.SkipOnModel(append(StableModels, "kevin")...)
