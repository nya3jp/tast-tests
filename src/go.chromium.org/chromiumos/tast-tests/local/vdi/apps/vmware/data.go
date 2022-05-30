// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vmware

// VmwareData holds the UI fragments that are used by Vmware connector. Use
// this as a data dependency when connecting to VMware.
var VmwareData = []string{
	"vmware/Splashscreen_AddBtn.png",
}

// UIFragmentName is the identifier to retrieve location of image from
// UiFragments.
type UIFragmentName int

const (
	// SplashscreenAddBtn is an id for retrieving path to the image.
	SplashscreenAddBtn UIFragmentName = iota
)
