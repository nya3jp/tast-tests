// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIPRoundedCornersUnderlay,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that ARC++ PIP rounded corners are implemented with a hardware underlay",
		Contacts:     []string{"amusbach@chromium.org", "oshima@chromium.org", "chromeos-perf@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "proprietary_codecs"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		Data:         []string{"bear-320x240.h264.mp4"},
		Fixture:      "gpuWatchDog",
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func PIPRoundedCornersUnderlay(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	if err := a.Install(ctx, arc.APKPath("ArcPipVideoTest.apk")); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, "org.chromium.arc.testapp.pictureinpicturevideo", ".VideoActivity")
	if err != nil {
		s.Fatal("Failed to create activity: ", err)
	}
	defer act.Close()

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		s.Fatal("Failed to parse test server URL: ", err)
	}

	hostPort, err := strconv.Atoi(srvURL.Port())
	if err != nil {
		s.Fatal("Failed to parse test server port: ", err)
	}

	androidPort, err := a.ReverseTCP(ctx, hostPort)
	if err != nil {
		s.Fatal("Failed to start reverse port forwarding: ", err)
	}
	defer a.RemoveReverseTCP(ctx, androidPort)

	if err := act.Start(ctx, tconn, arc.WithExtraString("video_uri", fmt.Sprintf("http://localhost:%d/bear-320x240.h264.mp4", androidPort))); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer act.Stop(cleanupCtx, tconn)

	// The test activity enters PIP mode in onUserLeaveHint().
	if err := act.SetWindowState(ctx, tconn, arc.WindowStateMinimized); err != nil {
		s.Fatal("Failed to minimize app: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.State == ash.WindowStatePIP }); err != nil {
			return errors.Wrap(err, "the PIP window hasn't been created yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for PIP window: ", err)
	}

	if err := d.WaitForIdle(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for app to idle: ", err)
	}

	hists, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait a second")
		}
		return nil
	}, "Viz.DisplayCompositor.OverlayStrategy")
	if err != nil {
		s.Fatal("Failed to record histogram Viz.DisplayCompositor.OverlayStrategy: ", err)
	}

	hist := hists[0]
	if len(hist.Buckets) == 0 {
		s.Fatal("Got no overlay strategy data")
	}

	for _, bucket := range hist.Buckets {
		// bucket.Min will be from enum OverlayStrategies as defined
		// in tools/metrics/histograms/enums.xml in the chromium
		// code base. We want the PIP video promoted to overlay with
		// the underlay overlay strategy (4) or not at all (1,6,7).
		if bucket.Min != 1 && bucket.Min != 4 && bucket.Min != 6 && bucket.Min != 7 {
			s.Errorf("Found %d frame(s) with an unexpected overlay strategy: got %d; want 1, 4, 6, or 7", bucket.Count, bucket.Min)
		}
	}
}
