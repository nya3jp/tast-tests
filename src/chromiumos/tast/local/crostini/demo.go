// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"image/color"

	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
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

// CloseDemoWithKeyboard issues a keystroke to the device (which our demos use
// a shutdown signal). This method of terminating the demo is preferred as it
// allows us to see log messages/errors printed by the demo app.
func CloseDemoWithKeyboard(ctx context.Context, s *testing.State) {
	s.Log("Closing the demo app with a keypress")
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Error("Failed to find keyboard device: ", err)
	}
	defer keyboard.Close()

	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}
}
