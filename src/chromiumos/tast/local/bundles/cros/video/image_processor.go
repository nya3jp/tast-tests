// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const imageProcessorUnitTestBin = "image_processor_test"

func init() {
	testing.AddTest(&testing.Test{
		Func: ImageProcessor,
		Desc: "Runs ImageProcessor unit tests",
		Contacts: []string{
			"nhebert@google.com",
			"chromeos-gfx-video@google.com",
		},
		Attr:    []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Fixture: "graphicsNoChrome",
		Params: []testing.Param{
			{
				Name:    "image_processor_unit_test",
				Timeout: 5 * time.Minute,
				ExtraData: []string{
					"images/bear_192x320_270.nv12.yuv",
					"images/bear_192x320_270.nv12.yuv.json",
					"images/bear_192x320_90.nv12.yuv",
					"images/bear_192x320_90.nv12.yuv.json",
					"images/bear_320x192_180.nv12.yuv",
					"images/bear_320x192_180.nv12.yuv.json",
					"images/bear_320x192.bgra",
					"images/bear_320x192.bgra.json",
					"images/bear_320x192.i420.yuv",
					"images/bear_320x192.i420.yuv.json",
					"images/bear_320x192.i422.yuv",
					"images/bear_320x192.i422.yuv.json",
					"images/bear_320x192.nv12.yuv",
					"images/bear_320x192.nv12.yuv.json",
					"images/bear_320x192.rgba",
					"images/bear_320x192.rgba.json",
					"images/bear_320x192.yuyv.yuv",
					"images/bear_320x192.yuyv.yuv.json",
					"images/bear_320x192.yv12.yuv",
					"images/bear_320x192.yv12.yuv.json",
					"images/puppets-1280x720.nv12.yuv",
					"images/puppets-1280x720.nv12.yuv.json",
					"images/puppets-320x180.nv12.yuv",
					"images/puppets-320x180.nv12.yuv.json",
					"images/puppets-480x270.i420.yuv",
					"images/puppets-480x270.i420.yuv.json",
					"images/puppets-480x270.nv12.yuv",
					"images/puppets-480x270.nv12.yuv.json",
					"images/puppets-640x360.i420.yuv",
					"images/puppets-640x360.i420.yuv.json",
					"images/puppets-640x360.i422.yuv",
					"images/puppets-640x360.i422.yuv.json",
					"images/puppets-640x360_in_640x480.nv12.yuv",
					"images/puppets-640x360_in_640x480.nv12.yuv.json",
					"images/puppets-640x360.nv12.yuv",
					"images/puppets-640x360.nv12.yuv.json",
					"images/puppets-640x360.yuyv.yuv",
					"images/puppets-640x360.yuyv.yuv.json",
				},
				Val: "*",
			},
		},
	})
}

// ImageProcessor runs image_processor_test binary and checks for errors.
// For some V4L2 platforms, the GPU is used. For others, libyuv is used.
// TODO(nhebert): Add platform specific ImageProcessor controls.
func ImageProcessor(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to create new video logger: ", err)
	}
	defer vl.Close()

	dataDirectory := filepath.Dir(s.DataPath("images/bear_320x192.rgba"))

	testName := s.Param().(string)

	s.Logf("Running %s", imageProcessorUnitTestBin)

	testArgs := []string{fmt.Sprintf("--source_directory=%s", dataDirectory),
		logging.ChromeVmoduleFlag()}

	gtestFilter := gtest.Filter(testName)

	exec := filepath.Join(chrome.BinTestDir, imageProcessorUnitTestBin)
	logfile := filepath.Join(s.OutDir(),
		fmt.Sprintf("output_%s_%d.txt", filepath.Base(exec), time.Now().Unix()))
	t := gtest.New(exec, gtest.Logfile(logfile),
		gtestFilter,
		gtest.ExtraArgs(testArgs...),
		gtest.UID(int(sysutil.ChronosUID)))

	command, err := t.Args()
	if err != nil {
		s.Fatal("Failed to get gtest args: ", err)
	}
	testing.ContextLogf(ctx, "Running %s", shutil.EscapeSlice(command))

	if report, err := t.Run(ctx); err != nil {
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
		s.Fatalf("Failed to run %v: %v", exec, err)
	}
}
