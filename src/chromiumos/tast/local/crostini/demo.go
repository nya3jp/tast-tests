// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"image/color"

	"chromiumos/tast/local/colorcmp"
)

// The DemoConfig object holds a configuration for running a tast test that uses one of the demo applications.
type DemoConfig struct {
	Name          string
	AppPath       string
	DominantColor color.Color
}

// X11DemoConfig is the configuration needed to run demo tests based on the x11_demo app.
var X11DemoConfig = &DemoConfig{
	Name:          "x11",
	AppPath:       "/opt/google/cros-containers/bin/x11_demo",
	DominantColor: colorcmp.RGB(0x99, 0xee, 0x44),
}

// WaylandDemoConfig is the configuration needed to run demo tests based on the wayland_demo app.
var WaylandDemoConfig = &DemoConfig{
	Name:          "wayland",
	AppPath:       "/opt/google/cros-containers/bin/wayland_demo",
	DominantColor: colorcmp.RGB(0x33, 0x88, 0xdd),
}
