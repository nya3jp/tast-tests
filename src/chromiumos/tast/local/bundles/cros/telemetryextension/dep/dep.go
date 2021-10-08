// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dep contains hardware dependencies for Telemetry Extension related tests.
package dep

import (
	"chromiumos/tast/testing/hwdep"
)

// This list initially copied from
// https://docs.google.com/spreadsheets/d/1VbP5Z3788z2R8C4nO0Dw4lhMxvXnQw6m21WBYC5RjVE/edit?resourcekey=0-duzL5OE72_zmzlrhvAuDaQ#gid=0
// We are targeting only allowlisted OEMs and models for Telemetry Extension due to pricacy reasons.
//
// Eventually all device models of allowlisted OEMs should appear in this list.
var targetModelList = []string{
	// HP devices:

	// "dedede" board:
	"drawcia",
	"drawlat",
	"madoo",

	// "grunt" board:
	"barla",
	"careena",
	"mordin",

	// "hatch" board:
	"dragonair",
	"dratini",
	"jinlon",
	"stormfly",

	// "jacuzzi" board:
	"esche",
	"kappa",

	// "kip" board:
	"kip",
	"kip14",

	// "nami" board:
	"sona",
	"syndra",

	// "octopus" board:
	"bipship",
	"bloog",
	"blooglet",
	"blooguard",
	"dorp",
	"meep",
	"mimrock",
	"vorticon",
	"vortininja",

	// "relm" board:
	"locke",

	// "setzer" board:
	"setzer",

	// "snappy" board:
	"alan",
	"bigdaddy",
	"snappy",
}

// TargetModels returns hardwareDeps condition with list of models aiming to pass Telemetry Extension tests.
func TargetModels() hwdep.Deps {
	return hwdep.D(hwdep.Model(targetModelList...))
}

// NonTargetModels return hardwareDeps condition with list of models not aiming to pass Telemetry Extension tests,
// however models from this list thas pass Telemetry Extension tests will be included in target model list.
func NonTargetModels() hwdep.Deps {
	return hwdep.D(hwdep.SkipOnModel(targetModelList...))
}
