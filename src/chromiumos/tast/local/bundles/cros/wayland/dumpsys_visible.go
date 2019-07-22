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

// Each layer description line looks as follows
// Layer ${ID} Z: ${Z} visible: ${VISIBLE} hidden ${HIDDEN} ...
const (
	idfmt      = `Layer (?P<id>0x[0-9a-f]{8})`
	zfmt       = `Z:\s+(?P<z>\d+)`
	visiblefmt = `visible:\s+(?P<visible>\d)`
	hiddenfmt  = `hidden:\s+(?P<hidden>\d)`
	layerfmt   = idfmt + `\s+` + zfmt + `\s+` + visiblefmt + `\s+` + hiddenfmt
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DumpsysVisible,
		Desc:         "Checks dumpsys Wayland output of only visible layers",
		Contacts:     []string{"walczakm@google.com", "sarakato@google.com", "tetsui@google.com", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

// DumpsysVisible checks that output of `dumpsys Wayland --visible`
// shows only visible layers.
func DumpsysVisible(ctx context.Context, s *testing.State) {
	re := regexp.MustCompile(layerfmt)
	a := s.PreValue().(arc.PreData).ARC

	cmd := a.Command(ctx, "dumpsys", "Wayland", "--visible")
	out, err := cmd.Output()
	if err != nil {
		s.Fatal("Running `dumpsys Wayland --visible` failed: ", err)
	}

	layers := re.FindAllSubmatch(out, -1)
	if len(layers) == 0 {
		s.Fatal("Unable to find any layers in the output")
	}

	for _, layer := range layers {
		// Get matched ${VISIBLE} and check if it equals 1.
		visible := string(layer[3])
		if visible != "1" {
			s.Error("Unexpected visibility of layer ", string(layer[1]), ", expected: 1, got: ", visible)
		}
	}
}
