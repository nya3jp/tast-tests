// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package citrix

// CitrixData holds the UI fragments that are used by Citrix connector. It is
// needed to assign data dependency for fixture that use connection to Citrix.
var CitrixData = []string{
	"citrix/Splashscreen_ServerUrlTbx.png",
}

// UIFragmentName is the identifier to retrieve location of image from
// UiFragments.
type UIFragmentName int

const (
	// SplashscreenServerURLTbx is an id for retrieving path to the image.
	SplashscreenServerURLTbx UIFragmentName = iota
)

// UIFragments is a mapping of UiFragmentName to data file (UI fragment).
var UIFragments = map[UIFragmentName]string{
	SplashscreenServerURLTbx: CitrixData[SplashscreenServerURLTbx],
}
