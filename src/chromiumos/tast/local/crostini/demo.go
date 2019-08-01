// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"image/color"

	"chromiumos/tast/local/colorcmp"
)

// The DemoConfig object holds a configuration for running a tast test that
// uses one of the demo applications.
type DemoConfig struct {
	// Name identifies the demo configuration (e.g. "x11" is the demo which
	// verifies that we can render x11 windows).
	Name string
	// AppPath is the path to the demo's executable.
	AppPath string
	// DominantColot is used to paint the demo's window. By convention, we
	// associate demo windows with colors (in order to easily identify
	// screenshots).
	DominantColor color.Color
}

// X11DemoConfig returns the configuration needed to run demo tests based on the x11_demo app.
func X11DemoConfig() DemoConfig {
	return DemoConfig{
		Name:          "x11",
		AppPath:       "/opt/google/cros-containers/bin/x11_demo",
		DominantColor: colorcmp.RGB(0x99, 0xee, 0x44),
	}
}

// WaylandDemoConfig returns the configuration needed to run demo tests based on the wayland_demo app.
func WaylandDemoConfig() DemoConfig {
	return DemoConfig{
		Name:          "wayland",
		AppPath:       "/opt/google/cros-containers/bin/wayland_demo",
		DominantColor: colorcmp.RGB(0x33, 0x88, 0xdd),
	}
}
