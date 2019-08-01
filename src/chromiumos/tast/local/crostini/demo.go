// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"image/color"

	"chromiumos/tast/local/colorcmp"
)

const (
	// X11DemoAppPath is a path (in the container) to an application used for verifying simple X11 functionality.
	X11DemoAppPath = "/opt/google/cros-containers/bin/x11_demo"

	// WaylandDemoAppPath is a path (in the container) to an application used for verifying simple Wayland functionality.
	WaylandDemoAppPath = "/opt/google/cros-containers/bin/wayland_demo"
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
	AppPath:       X11DemoAppPath,
	DominantColor: colorcmp.RGB(0x99, 0xee, 0x44),
}

// WaylandDemoConfig is the configuration needed to run demo tests based on the wayland_demo app.
var WaylandDemoConfig = &DemoConfig{
	Name:          "wayland",
	AppPath:       WaylandDemoAppPath,
	DominantColor: colorcmp.RGB(0x33, 0x88, 0xdd),
}
