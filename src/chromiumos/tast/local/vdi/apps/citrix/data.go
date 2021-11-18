// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package citrix

// CitrixData holds the UI fragments that are used by Citrix connector. Use
// this as a data dependency when connecting to Citrix.
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
