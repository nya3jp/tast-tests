// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
var AppsStableModels = hwdep.D(hwdep.Model(stableModels...))

// AppsUnstableModels is a list of models to run inputs tests at 'informational' so that we know once they are stable enough to be promoted to CQ.
// kevin64 is an experimental board does not support nacl, which fails Canvas installation.
// To stablize the tests, have to exclude entire kevin model as no distinguish between kevin and kevin64.
var AppsUnstableModels = hwdep.D(hwdep.SkipOnModel(append(stableModels, "kevin1")...))

// TODO(crbug.com/1137646): Remove this list once crbug.com/2459968 has been fixed.
var systemWebAppsUnstableModels = []string{
	"gandof",
	"guado",
	"lulu",
	"paine",
	"samus",
	"tidus",
	"yuna",
}

// SystemWebAppsStableModels is a list of models that are stable to be promoted to critical.
var SystemWebAppsStableModels = hwdep.D(hwdep.SkipOnModel(systemWebAppsUnstableModels...))

// SystemWebAppsUnstableModels is a list of models that are flaky as Terminal SWA is not reinstalled due to crbug.com/2459968.
var SystemWebAppsUnstableModels = hwdep.D(hwdep.Model(systemWebAppsUnstableModels...))
