// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/binsetup"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

type decoderConfig struct {
	format      string
	gtestFilter string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VAAPIUnittest,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies VA-API utility and image decode acceleration functionality",
		Contacts: []string{
			"andrescj@chromium.org", // JPEG decoder test maintainer
			"gildekel@chromium.org", // WebP decoder test author
			"chromeos-gfx@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "no_qemu", "vaapi"}, // TODO(crbug.com/1080871): Remove no_qemu SoftwareDeps.
		Params: []testing.Param{{
			Name: "webp_decoder",
			Val: decoderConfig{
				format:      "webp",
				gtestFilter: webpGFilter,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         vaImageTestFiles["webp"],
		}, {
			Name: "jpeg_decoder",
			Val: decoderConfig{
				format:      "jpeg",
				gtestFilter: jpegGFilter,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeJPEG},
			ExtraData:         vaImageTestFiles["jpeg"],
		}, {
			Name: "common",
			Val: decoderConfig{
				format:      "common",
				gtestFilter: fmt.Sprintf("-%s:%s", webpGFilter, jpegGFilter),
			},
		}},
		Fixture: "graphicsNoChrome",
	})
}

const (
	webpGFilter = "*VaapiWebPDecoderTest.*"
	jpegGFilter = "*VaapiJpegDecoder*Test.*"
)

var vaImageTestFiles = map[string][]string{
	"jpeg": {
		"pixel-1280x720.jpg",
		"pixel-1280x720-grayscale.jpg",
		"pixel-1280x720-yuv420.jpg",
		"pixel-1280x720-yuv444.jpg",
		"pixel-40x23-yuv420.jpg",
		"pixel-41x22-yuv420.jpg",
		"pixel-41x23-yuv420.jpg",
	},
	"webp": {
		"BlackAndWhite_criss-cross_pattern_2015x2015.webp",
		"RGB_noise_2015x2015.webp",
		"RGB_noise_large_pixels_115x115.webp",
		"RGB_noise_large_pixels_2015x2015.webp",
		"RGB_noise_large_pixels_4000x4000.webp",
		"solid_green_2015x2015.webp",
	},
}

// VAAPIUnittest runs a set of HW accelerated decode tests, defined in
// vaapi_unittest.
func VAAPIUnittest(ctx context.Context, s *testing.State) {
	// The VA-API decode test operates on all files in a single directory.
	// testing.State doesn't guarantee that all data files will be stored in the same
	// directory, so copy them to a temp dir.
	decoderVal := s.Param().(decoderConfig)
	var srcs []string
	for _, fn := range vaImageTestFiles[decoderVal.format] {
		srcs = append(srcs, s.DataPath(fn))
	}

	tempDir, err := binsetup.CreateTempDataDir(fmt.Sprintf("VAAPIUnittest.tast.%s", decoderVal.format), srcs)
	if err != nil {
		s.Fatal("Failed to create a temporary directory: ", err)
	}
	defer os.RemoveAll(tempDir)

	// Execute the test binary.
	const exec = "vaapi_unittest"
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.ExtraArgs("--test_data_path="+tempDir+"/"),
		gtest.Filter(decoderVal.gtestFilter),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		} else {
			s.Error("No additional information is available for this failure")
		}
	}
}
