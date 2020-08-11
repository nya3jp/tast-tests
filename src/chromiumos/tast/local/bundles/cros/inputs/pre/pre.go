// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains preconditions for inputs tests.
package pre

// InputsCriticalModels is a shortlist of boards aiming to run critical inputs tests.
// More information refers to http://b/161415599.
var InputsCriticalModels = []string{
	// Top VK usage board in 2020 -- convertible, ARM.
	"hana",
	// Another top board -- convertible, x64.
	"snappy",
	// Not much usage, but very small tablet.
	"kukui",
	// Convertible chromebook, top usage in 2018 and 2019.
	"cyan",
	// Random boards on the top boards for VK list.
	"octopus",
	"kefka",
	"coral",
	// VM used for basic development.
	"betty",
}

// InputsTabletModels is a shortlist of physical tablet models to run critical inputs tests.
var InputsTabletModels = []string{
	"krane",
	"dru",
	"dumo",
	"kukui",
	"nocturne",
}
