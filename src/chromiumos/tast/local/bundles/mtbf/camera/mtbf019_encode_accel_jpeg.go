// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

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
		Func:         MTBF019EncodeAccelJPEG,
		Desc:         "Run Chrome jpeg_encode_accelerator_unittest",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeJPEG},
		Data:         []string{"bali_640x368_P420.yuv"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF019EncodeAccelJPEG runs a set of HW JPEG encode tests, defined in
// jpeg_encode_accelerator_unittest.
func MTBF019EncodeAccelJPEG(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoLogging, err))
	}
	defer vl.Close()

	// Execute the test binary.
	const exec = "jpeg_encode_accelerator_unittest"

	test := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), "gtest.log")),
		gtest.ExtraArgs(
			logging.ChromeVmoduleFlag(),
			fmt.Sprintf("--yuv_filenames=%s:640x368", s.DataPath("bali_640x368_P420.yuv"))),
		gtest.UID(int(sysutil.ChronosUID)),
	)
	const loopTimes int = 20
	for i := 0; i < loopTimes; i++ {
		if report, err := test.Run(ctx); err != nil {
			if strings.Contains(err.Error(), "segmentation fault") {
				s.Fatal(mtbferrors.New(mtbferrors.SegmentationFault, err, exec))
			}
			s.Fatal(mtbferrors.New(mtbferrors.VideoUTRun, err, exec))
			if report != nil {
				for _, name := range report.FailedTestNames() {
					s.Fatal(mtbferrors.New(mtbferrors.VideoUTFailure, err, name))
				}
			}
		}
		testing.Sleep(ctx, time.Second)
	}
}
