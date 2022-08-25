// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

type VideoSmoothnessData struct {
	FramesExpected int   `json:"framesExpected"`
	FramesProduced int   `json:"framesProduced"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoSmoothness,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Demo of the aggregate video smoothness metrics API",
		Contacts:     []string{"hob@chromium.org"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome", "android_p"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Timeout:      cuj.CPUStablizationTimeout + 100 * time.Second,
		Pre:          arc.Booted(),
	})
}

func VideoSmoothness(ctx context.Context, s *testing.State) {
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.PreValue().(arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Unable to create test connection: ", err)
	}

	testing.Sleep(ctx, 1 * time.Second)

	// Open a browser window with a single video playing and dock left.
	conn, err := cr.NewConn(ctx,
		"http://chrome.brkho.com/onevideo.html",
		browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open single video page: ", err)
	}
	defer conn.Close()
	window, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find active window: ", err)
	}
	if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateLeftSnapped); err != nil {
		s.Fatal("Failed to snap left: ", err)
	}

	// Open a browser window with two videos playing and dock right.
	conn2, err := cr.NewConn(ctx,
		"http://chrome.brkho.com/twovideos.html",
		browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open two videos page: ", err)
	}
	defer conn2.Close()
	window, err = ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find active window: ", err)
	}
	if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateRightSnapped); err != nil {
		s.Fatal("Failed to snap right: ", err)
	}

	// Begin tracking, take N measurements, and end tracking.
	if err := tconn.Eval(ctx, `chrome.autotestPrivate.beginVideoSmoothnessTracking();`, nil); err != nil {
		s.Fatal("Unable to begin smoothness tracking: ", err)
	}

	for i := 0; i < 10; i++ {
		testing.Sleep(ctx, 5 * time.Second)
		{
			var data VideoSmoothnessData
			err = tconn.Call(ctx, &data, `tast.promisify(chrome.autotestPrivate.getVideoSmoothness)`)
			if err != nil {
				s.Fatal("Unable to get smoothness data", err)
			}
			s.Log(data)
		}
	}

	if err := tconn.Eval(ctx, `chrome.autotestPrivate.endVideoSmoothnessTracking();`, nil); err != nil {
		s.Fatal("Unable to end smoothness tracking: ", err)
	}
	testing.Sleep(ctx, 300 * time.Second)
}
