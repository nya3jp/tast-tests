// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HardwareProbe,
		Desc: "Verify hardware_probe binary can detect various information",
		Contacts: []string{
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr:    []string{"group:graphics", "graphics_drm", "group:mainline", "informational"},
		Fixture: "gpuWatchDog",
	})
}

// HardwareProbe verifies we can successfully retrieve various device information via hardware_probe.
func HardwareProbe(ctx context.Context, s *testing.State) {
	file := filepath.Join(s.OutDir(), "hardware_probe.json")
	result, err := graphics.GetHardwareProbeResult(ctx, file)
	if err != nil {
		s.Fatal("Failed to run hardware_probe: ", err)
	}
	s.Log("Successfully get the information: ", result)
}
