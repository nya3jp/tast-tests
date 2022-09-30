// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dep

import (
	"chromiumos/tast/testing/hwdep"
)

// List of models that pass the telemetryextension.HasOEMName test.
// In general it's expected that Telemetry Extension Platform tests will pass
// on these models.
var nonStableModelList = []string{
	// HP models:
	"alan",
	"anahera",
	"barla",
	"berknip",
	"bigdaddy",
	"bipship",
	"bloog",
	"blooglet",
	"blooguard",
	"burnet",
	"careena",
	"dirinboz",
	"dorp",
	"dragonair",
	"dratini",
	"drawcia",
	"drawlat",
	"drawman",
	"drawper",
	"gumboz",
	"jinlon",
	"landia",
	"landrid",
	"locke",
	"madoo",
	"meep",
	"mordin",
	"nipperkin",
	"noibat",
	"setzer",
	"snappy",
	"sona",
	"stormfly",
	"syndra",
	"vorticon",
	"vortininja",
}

// NonStableModels returns hardwareDeps condition with list of non-stable models.
func NonStableModels() hwdep.Deps {
	return hwdep.D(hwdep.Model(nonStableModelList...))
}
