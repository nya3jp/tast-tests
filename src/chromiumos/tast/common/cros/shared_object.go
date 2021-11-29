// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package core is used to control Chrome OS Nearby Share functionality.
package core

import (
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
)

type SharedObjectsForService struct {
	Chrome *chrome.Chrome
	Arc    *arc.ARC
}

// SharedObjectsForServiceSingleton allows sharing states between services
var SharedObjectsForServiceSingleton = &SharedObjectsForService{}
