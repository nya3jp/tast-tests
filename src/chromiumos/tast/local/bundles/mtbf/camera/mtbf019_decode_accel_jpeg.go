// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF019DecodeAccelJPEG,
		Desc:         "Run Chrome jpeg_decode_accelerator_unittest",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeJPEG},
		Data:         []string{decodeAccelJpegTestFile},
		Pre:          chrome.LoginReuse(),
	})
}

const decodeAccelJpegTestFile = "peach_pi-1280x720.jpg"

// MTBF019DecodeAccelJPEG runs a set of HW JPEG decode tests, defined in
// jpeg_decode_accelerator_unittest.
func MTBF019DecodeAccelJPEG(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoLogging, err))
	}
	defer vl.Close()

	testDir := filepath.Dir(s.DataPath(decodeAccelJpegTestFile))

	// Execute the test binary.
	const exec = "jpeg_decode_accelerator_unittest"
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.ExtraArgs(
			logging.ChromeVmoduleFlag(),
			"--test_data_path="+testDir+"/",
			"--jpeg_filenames="+decodeAccelJpegTestFile),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoUTRun, err, exec))
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Fatal(mtbferrors.New(mtbferrors.VideoUTFailure, err, name))
			}
		}
	}
}
