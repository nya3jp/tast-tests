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
	"barla",
	"bigdaddy",
	"bipship",
	"bloog",
	"blooglet",
	"blooguard",
	"careena",
	"dorp",
	"dragonair",
	"dratini",
	"drawcia",
	"drawlat",
	"jinlon",
	"locke",
	"madoo",
	"meep",
	"mordin",
	"redrix",
	"setzer",
	"snappy",
	"sona",
	"stormfly",
	"syndra",
}

// NonStableModels returns hardwareDeps condition with list of non-stable models.
func NonStableModels() hwdep.Deps {
	return hwdep.D(hwdep.Model(nonStableModelList...))
}
