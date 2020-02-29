// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DisplayDensity,
		Desc:     "Runs a crostini application from the terminal in high/low DPI modes and compares sizes",
		Contacts: []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline"},
		Params: []testing.Param{{
			Name:              "wayland_artifact",
			Pre:               crostini.StartedByArtifact(),
			Timeout:           7 * time.Minute,
			ExtraData:         []string{crostini.ImageArtifact},
			Val:               crostini.WaylandDemoConfig(),
			ExtraSoftwareDeps: []string{"crostini_stable"},
		}, {
			Name:              "wayland_artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			Timeout:           7 * time.Minute,
			ExtraData:         []string{crostini.ImageArtifact},
			Val:               crostini.WaylandDemoConfig(),
			ExtraSoftwareDeps: []string{"crostini_unstable"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "wayland_download_buster",
			Pre:       crostini.StartedByDownloadBuster(),
			Timeout:   10 * time.Minute,
			Val:       crostini.WaylandDemoConfig(),
			ExtraAttr: []string{"informational"},
		}, {
			Name:              "x11_artifact",
			Pre:               crostini.StartedByArtifact(),
			Timeout:           7 * time.Minute,
			ExtraData:         []string{crostini.ImageArtifact},
			Val:               crostini.X11DemoConfig(),
			ExtraSoftwareDeps: []string{"crostini_stable"},
		}, {
			Name:              "x11_artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			Timeout:           7 * time.Minute,
			ExtraData:         []string{crostini.ImageArtifact},
			Val:               crostini.X11DemoConfig(),
			ExtraSoftwareDeps: []string{"crostini_unstable"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "x11_download_buster",
			Pre:       crostini.StartedByDownloadBuster(),
			Timeout:   10 * time.Minute,
			Val:       crostini.X11DemoConfig(),
			ExtraAttr: []string{"informational"},
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func DisplayDensity(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	tconn := pre.TestAPIConn
	cont := pre.Container
	keyboard := pre.Keyboard
	conf := s.Param().(crostini.DemoConfig)

	type density int

	const (
		lowDensity density = iota
		highDensity
	)

	demoWindowSize := func(densityConfiguration density) (coords.Size, error) {
		windowName := conf.Name
		var subCommandArgs []string
		if densityConfiguration == lowDensity {
			windowName = windowName + "_low_density"
			// TODO(hollingum): Find a better way to pass environment vars to a container command (rather than invoking sh).
			subCommandArgs = append(subCommandArgs, "DISPLAY=${DISPLAY_LOW_DENSITY}", "WAYLAND_DISPLAY=${WAYLAND_DISPLAY_LOW_DENSITY}")
		}
		subCommandArgs = append(subCommandArgs, conf.AppPath, "--width=100", "--height=100", "--title="+windowName)

		cmd := cont.Command(ctx, "sh", "-c", strings.Join(subCommandArgs, " "))
		s.Logf("Running %q", shutil.EscapeSlice(cmd.Args))
		if err := cmd.Start(); err != nil {
			return coords.Size{}, err
		}
		defer cmd.Wait(testexec.DumpLogOnError)
		defer cmd.Kill()

		var sz coords.Size
		var err error
		if sz, err = crostini.PollWindowSize(ctx, tconn, windowName); err != nil {
			return coords.Size{}, err
		}
		s.Logf("Window %q size is %v", windowName, sz)

		s.Logf("Closing %q with keypress", windowName)
		err = keyboard.Accel(ctx, "Enter")

		return sz, err
	}

	sizeHighDensity, err := demoWindowSize(highDensity)
	if err != nil {
		s.Fatal("Failed getting high-density window size: ", err)
	}

	sizeLowDensity, err := demoWindowSize(lowDensity)
	if err != nil {
		s.Fatal("Failed getting low-density window size: ", err)
	}

	if err := crostini.VerifyWindowDensities(ctx, tconn, sizeHighDensity, sizeLowDensity); err != nil {
		s.Fatal("Failed during window density comparison: ", err)
	}
}
