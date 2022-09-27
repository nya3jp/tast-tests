// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dep

import (
	"chromiumos/tast/testing/hwdep"
)

// This list was fetched by Plx script, use "HP" as primaryOemName:
// https://plx.corp.google.com/scripts2/script_61._62eca9_0000_2e5b_930b_30fd38187cc4
//
// Also manually added next models:
//   - agah
//   - joxer
//   - joxton
//   - vell
//   - vorticon
//   - vortininja
var hpModelList = []string{
	"agah",
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
	"chell",
	"coachz",
	"dirinboz",
	"dojo",
	"dooly",
	"dorp",
	"dragonair",
	"dratini",
	"drawcia",
	"drawlat",
	"drawman",
	"drawper",
	"eldrid",
	"elemi",
	"esche",
	"gimble",
	"giygas",
	"gumboz",
	"habokay",
	"haboki",
	"jinlon",
	"joxer",
	"joxton",
	"kappa",
	"kench",
	"kingoftown",
	"landia",
	"landrid",
	"lantis",
	"locke",
	"madoo",
	"meep",
	"mordin",
	"nipperkin",
	"noibat",
	"redrix",
	"setzer",
	"snappy",
	"sona",
	"soraka",
	"stormcutter",
	"stormfly",
	"sylas",
	"syndra",
	"vell",
	"vorticon",
	"vortininja",
}

// HPModels returns hardwareDeps condition with list of all HP models.
func HPModels() hwdep.Deps {
	return hwdep.D(hwdep.Model(hpModelList...))
}
