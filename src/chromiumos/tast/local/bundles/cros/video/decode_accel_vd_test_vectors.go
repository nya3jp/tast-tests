// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/decoding"
	"chromiumos/tast/testing"
)

func getTestFiles() []string {
	// TODO(hiroh): List up automatically.
	return []string{
		"test_vectors/av1/00000548.ivf",
		"test_vectors/av1/00000548.ivf.json",
		"test_vectors/av1/48_delayed.ivf",
		"test_vectors/av1/48_delayed.ivf.json",
		"test_vectors/av1/av1-1-b8-02-allintra.ivf",
		"test_vectors/av1/av1-1-b8-02-allintra.ivf.json",
		"test_vectors/av1/av1-1-b8-03-sizeup.ivf",
		"test_vectors/av1/av1-1-b8-03-sizeup.ivf.json",
		"test_vectors/av1/av1-1-b8-23-film_grain-50.ivf",
		"test_vectors/av1/av1-1-b8-23-film_grain-50.ivf.json",
		"test_vectors/av1/ccvb_film_grain.ivf",
		"test_vectors/av1/ccvb_film_grain.ivf.json",
		"test_vectors/av1/crosvideo_last_2sec.ivf",
		"test_vectors/av1/crosvideo_last_2sec.ivf.json",
		"test_vectors/av1/frames_refs_short_signaling.ivf",
		"test_vectors/av1/frames_refs_short_signaling.ivf.json",
		"test_vectors/av1/non_uniform_tiling.ivf",
		"test_vectors/av1/non_uniform_tiling.ivf.json",
	}
}

func getIVFFiles() []string {
	files := getTestFiles()
	var ivfFiles []string
	for _, file := range files {
		if filepath.Ext(file) == ".ivf" {
			ivfFiles = append(ivfFiles, file)
		}
	}
	return ivfFiles
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelVDTestVectors,
		Desc:         "Verifies hardware decode acceleration of media::VideoDecoders by running the video_decode_accelerator_tests binary (see go/vd-migration)",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", "cros_video_decoder"},
		Params: []testing.Param{{
			Name:              "av1",
			Val:               getIVFFiles(),
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         getTestFiles(),
		}},
	})
}

func DecodeAccelVDTestVectors(ctx context.Context, s *testing.State) {
	var ivfFiles []string
	for _, file := range s.Param().([]string) {
		ivfFiles = append(ivfFiles, s.DataPath(file))
	}

	if err := decoding.RunAccelVideoTestToTestVectors(ctx, s.OutDir(), ivfFiles); err != nil {
		s.Fatal("test failed: ", err)
	}
}
