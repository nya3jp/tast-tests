// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/cuj"
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

type VideoSmoothnessRecord struct {
	TimestampMs int   `json:"timestampMs"`
	FramesExpected int   `json:"framesExpected"`
	FramesProduced int   `json:"framesProduced"`
}

type SingleVideoSmoothness struct {
	Records []VideoSmoothnessRecord   `json:"records"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoSmoothness,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Demo of the video smoothness metrics API",
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
	if err := tconn.Eval(ctx, `chrome.autotestPrivate.beginVideoSmoothnessTracking(5000);`, nil); err != nil {
		s.Fatal("Unable to begin smoothness tracking: ", err)
	}

	testing.Sleep(ctx, 30 * time.Second)

	var data []*SingleVideoSmoothness
	err = tconn.Call(ctx, &data, `tast.promisify(chrome.autotestPrivate.endVideoSmoothnessTracking)`)
	if err != nil {
		s.Fatal("Unable to end smoothness tracking: ", err)
	}
	for _, video := range data {
		s.Log("Video:")
		for _, record := range video.Records {
			s.Log(record)
		}
	}
	testing.Sleep(ctx, 300 * time.Second)
}
