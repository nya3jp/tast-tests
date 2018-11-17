// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelJPEG,
		Desc:         "Run Chrome jpeg_decode_accelerator_unittest",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeJPEG},
		Data:         jpegTestFiles,
	})
}

// jpegTestFiles lists the files required by the JPEG decode accelerator test.
var jpegTestFiles = []string{
	"peach_pi-1280x720.jpg",
	"peach_pi-40x23.jpg",
	"peach_pi-41x22.jpg",
	"peach_pi-41x23.jpg",
	"pixel-1280x720.jpg",
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
	// Copy the data files to a temp dir where they can be accessed by the test.
	tempDir, err := ioutil.TempDir("", "DecodeAccelJPEG.tast.")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(tempDir)
	if err := os.Chmod(tempDir, 0755); err != nil {
		s.Fatalf("Failed to chmod %v: %v", tempDir, err)
	}
	for _, f := range jpegTestFiles {
		src := s.DataPath(f)
		dst := filepath.Join(tempDir, f)
		if err := fsutil.CopyFile(src, dst); err != nil {
			s.Fatalf("Failed to copy test file %s to %s: %v", src, dst, err)
		}
		if err := os.Chmod(dst, 0644); err != nil {
			s.Fatalf("Failed to chmod %v: %v", dst, err)
		}
	}

	// Execute the test binary.
	args := []string{logging.ChromeVmoduleFlag(), "--test_data_path=" + tempDir + "/"}
	const exec = "jpeg_decode_accelerator_unittest"
	if err := bintest.Run(ctx, exec, args, s.OutDir()); err != nil {
		s.Fatalf("Failed to run %v: %v", exec, err)
	}
}
