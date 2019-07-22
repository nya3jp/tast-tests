// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wayland

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

// Each layer description line looks like follows
// Layer ${ID} Z: ${Z} visible: ${VISIBLE} hidden ${HIDDEN} ...
const (
	idfmt      = `Layer (?P<id>0x[0-9a-f]{8})`
	zfmt       = `Z:\s+(?P<z>\d+)`
	visiblefmt = `visible:\s+(?P<visible>\d)`
	hiddenfmt  = `hidden:\s+(?P<hidden>\d)`
	layerfmt   = idfmt + `\s+` + zfmt + `\s+` + visiblefmt + `\s+` + hiddenfmt
)

var layerRe = regexp.MustCompile(layerfmt)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DumpsysVisible,
		Desc:         "Checks Wayland dumpsys output of only visible layers",
		Contacts:     []string{"arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      2 * time.Minute,
	})
}

// DumpsysVisible checks if the output of dumpsys with --visible flag
// shows only layers with visible parameter set to true
func DumpsysVisible(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	cmd := a.Command(ctx, "dumpsys", "Wayland", "--visible")
	out, err := cmd.Output()
	if err != nil {
		s.Fatal("Running `dumpsys Wayland --visible` failed: ", err)
	}

	layers := layerRe.FindAllSubmatch(out, -1)
	if len(layers) == 0 {
		s.Fatal("Unable to find any layers in the output")
	}

	for _, layer := range layers {
		// Get matched ${VISIBLE} and check if it equals 1
		visible := string(layer[3])
		if visible != "1" {
			s.Error("Invalid layer visibility, expected: 1, got: ", visible)
		}
	}
}
