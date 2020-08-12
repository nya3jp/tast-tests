// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains preconditions for apps tests
package pre

// AppsCriticalModels is a shortlist of boards aiming to run critical essential apps tests.
var AppsCriticalModels = []string{
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
	// VM used for basic development.
	"betty",
}
