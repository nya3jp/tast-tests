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
	// TODO(crbug.com/1149750): List up automatically.
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

func getTestVectors() []string {
	var testVectors []string
	for _, file := range getTestFiles() {
		if filepath.Ext(file) == ".ivf" {
			testVectors = append(testVectors, file)
		}
	}
	return testVectors
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeCompliance,
		Desc:         "Verifies hardware decode acceleration of media::VideoDecoders by running the video_decode_accelerator_tests binary",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		SoftwareDeps: []string{"chrome", "video_decoder_direct"},
		Params: []testing.Param{{
			Name:              "av1_test_vectors",
			Val:               getTestVectors(),
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         getTestFiles(),
		}},
	})
}

func DecodeCompliance(ctx context.Context, s *testing.State) {
	var testVectors []string
	for _, file := range s.Param().([]string) {
		testVectors = append(testVectors, s.DataPath(file))
	}

	if err := decoding.RunAccelVideoTestWithTestVectors(ctx, s.OutDir(), testVectors); err != nil {
		s.Fatal("test failed: ", err)
	}
}
