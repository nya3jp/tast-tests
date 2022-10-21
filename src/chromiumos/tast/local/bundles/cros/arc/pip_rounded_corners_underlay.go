// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/arcpipvideotest"
	"chromiumos/tast/local/chrome"
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
		Attr:         []string{"group:mainline"},
		// Video playback doesn't work well on VM boards.
		SoftwareDeps: []string{"chrome", "no_qemu", "proprietary_codecs"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		Data:         []string{"180p_60fps_600frames.h264.mp4"},
		Fixture:      "gpuWatchDog",
		Timeout:      5 * time.Minute,
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

	hists, err := metrics.RunAndWaitAll(ctx, tconn, 3*time.Second, func(ctx context.Context) error {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		cleanUp, err := arcpipvideotest.EstablishARCPIPVideo(ctx, tconn, a, s.DataFileSystem(), false)
		if err != nil {
			return errors.Wrap(err, "failed to establish ARC PIP video")
		}
		cleanUp(cleanupCtx)
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
