// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/binsetup"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelJPEG,
		Desc:         "Run Chrome jpeg_decode_accelerator_unittest",
		Contacts:     []string{"henryhsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeJPEG},
		Data:         imageFiles,
	})
}

// imageFiles lists the files required by the JPEG decode accelerator test.
// TODO(crbug.com/986074): we should move the WebPs to a separate tast test.
// For now we make the WebP tests run with the jpeg_decode_accelerator_unittest binary.
// Also rename imageFiles back to jpegTestFiles once the tests are separated.
var imageFiles = []string{
	"BlackAndWhite_criss-cross_pattern_2015x2015.webp",
	"peach_pi-1280x720.jpg",
	"pixel-1280x720.jpg",
	"pixel-1280x720-grayscale.jpg",
	"pixel-1280x720-yuv420.jpg",
	"pixel-1280x720-yuv444.jpg",
	"pixel-40x23-yuv420.jpg",
	"pixel-41x22-yuv420.jpg",
	"pixel-41x23-yuv420.jpg",
	"RGB_noise_2015x2015.webp",
	"RGB_noise_large_pixels_115x115.webp",
	"RGB_noise_large_pixels_2015x2015.webp",
	"RGB_noise_large_pixels_4000x4000.webp",
	"solid_green_2015x2015.webp",
}

// DecodeAccelJPEG runs a set of HW JPEG decode tests, defined in
// jpeg_decode_accelerator_unittest.
func DecodeAccelJPEG(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// The JPEG decode test operates on all files in a single directory.
	// testing.State doesn't guarantee that all data files will be stored in the same
	// directory, so copy them to a temp dir.
	tempDir := binsetup.CreateTempDataDir(s, "DecodeAccelJPEG.tast.", imageFiles)
	defer os.RemoveAll(tempDir)

	// Execute the test binary.
	const exec = "jpeg_decode_accelerator_unittest"
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.ExtraArgs(logging.ChromeVmoduleFlag(), "--test_data_path="+tempDir+"/"),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
	}
}
