// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests lacros-chrome running on ChromeOS.
package lacros

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/lacros/gpucuj"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

const (
	traceConfigFile = "chrome_trace_cfg.pbtxt"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GpuCUJ,
		Desc:         "Lacros GPU performance CUJ tests",
		Contacts:     []string{"edcourtney@chromium.org", "hidehiko@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Timeout:      120 * time.Minute,
		Data:         []string{traceConfigFile, launcher.DataArtifact, "video.html", "bbb_1080p60_yuv.vp9.webm"},
		Params: []testing.Param{{
			Name: "maximized",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeMaximized,
				Rot90:    false,
			},
			Fixture: "lacrosStartedByData",
		}, {
			Name: "maximized_rot90",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeMaximized,
				Rot90:    true,
			},
			Fixture: "lacrosStartedByData",
		}, {
			Name: "maximized_composited",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeMaximized,
				Rot90:    false,
			},
			Fixture: "lacrosStartedByDataForceComposition",
		}, {
			Name: "threedot",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeThreeDot,
				Rot90:    false,
			},
			Fixture: "lacrosStartedByData",
		}, {
			Name: "threedot_rot90",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeThreeDot,
				Rot90:    true,
			},
			Fixture: "lacrosStartedByData",
		}, {
			Name: "threedot_composited",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeThreeDot,
				Rot90:    false,
			},
			Fixture: "lacrosStartedByDataForceComposition",
		}, {
			Name: "resize",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeResize,
				Rot90:    false,
			},
			Fixture: "lacrosStartedByData",
		}, {
			Name: "resize_rot90",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeResize,
				Rot90:    true,
			},
			Fixture: "lacrosStartedByData",
		}, {
			Name: "resize_composited",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeResize,
				Rot90:    false,
			},
			Fixture: "lacrosStartedByDataForceComposition",
		}, {
			Name: "moveocclusion",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeMoveOcclusion,
				Rot90:    false,
			},
			Fixture: "lacrosStartedByData",
		}, {
			Name: "moveocclusion_rot90",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeMoveOcclusion,
				Rot90:    true,
			},
			Fixture: "lacrosStartedByData",
		}, {
			Name: "moveocclusion_composited",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeMoveOcclusion,
				Rot90:    false,
			},
			Fixture: "lacrosStartedByDataForceComposition",
		}, {
			Name: "moveocclusion_withcroswindow",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeMoveOcclusionWithCrosWindow,
				Rot90:    false,
			},
			Fixture: "lacrosStartedByData",
		}, {
			Name: "moveocclusion_withcroswindow_rot90",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeMoveOcclusionWithCrosWindow,
				Rot90:    true,
			},
			Fixture: "lacrosStartedByData",
		}, {
			Name: "moveocclusion_withcroswindow_composited",
			Val: gpucuj.TestParams{
				TestType: gpucuj.TestTypeMoveOcclusionWithCrosWindow,
				Rot90:    false,
			},
			Fixture: "lacrosStartedByDataForceComposition",
		}},
	})
}

func GpuCUJ(ctx context.Context, s *testing.State) {
	// Setup server to serve video file.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	pv, cleanup, err := gpucuj.RunGpuCUJ(ctx, s.FixtValue().(launcher.FixtData),
		s.DataPath(launcher.DataArtifact), s.DataPath(traceConfigFile), s.Param().(gpucuj.TestParams), server.URL, s.OutDir())
	if err != nil {
		s.Fatal("Could not run GpuCUJ test: ", err)
	}
	defer func() {
		if err := cleanup(ctx); err != nil {
			s.Fatal("Failed to cleanup after creating test: ", err)
		}
	}()

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
