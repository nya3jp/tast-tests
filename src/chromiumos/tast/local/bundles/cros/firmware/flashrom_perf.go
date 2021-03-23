// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type params struct {
	region   string
	deadline float64
}

func init() {
	testing.AddTest(&testing.Test{
		Func: FlashromPerf,
		Desc: "Flashrom SPI flash E2E tests",
		Contacts: []string{
			"quasisec@chromium.org",        // Test Author
			"quasisec@chromium.org",        // CrOS Flashrom Maintainer
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"flashrom"},
		Params: []testing.Param{{
			Val: params{
				region:   "", // empty str implies a full read.
				deadline: 21000,
			},
		}, {
			Name: "fmap",
			Val: params{
				region:   "FMAP",
				deadline: 850,
			},
		}, {
			Name: "gbb",
			Val: params{
				region:   "GBB",
				deadline: 2700,
			},
		}, {
			Name: "rw_vpd",
			Val: params{
				region:   "RW_VPD",
				deadline: 2300,
			},
		}, {
			Name: "rw_elog",
			Val: params{
				region:   "RW_ELOG",
				deadline: 2300,
			},
		}, {
			Name: "coreboot",
			Val: params{
				region:   "COREBOOT",
				deadline: 6000,
			},
		}},
	})
}

// FlashromPerf runs the flashrom utility to verify various expected behaviour
// is maintained.  The function times Flashrom total execution time for a
// complete probe and read of a given region of the SPI flash.
func FlashromPerf(ctx context.Context, s *testing.State) {
	readTime := perf.Metric{
		Name:      "flashrom_region_read_time",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}
	p := s.Param().(params)

	perf := perf.NewValues()
	duration := testFlashromReadTime(ctx, s, p.region)
	perf.Set(readTime, duration)

	const lowerBound = 100 // sub-process execution should take some time.

	if err := perf.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
	if duration <= lowerBound || duration >= p.deadline {
		s.Errorf("Flashrom execution outside time-bounds, %v < expected < %v ms, got = %v ms", lowerBound, p.deadline, duration)
	}
}

func testFlashromReadTime(ctx context.Context, s *testing.State, region string) float64 {
	flashromStart := time.Now()

	opTempFile, err := ioutil.TempFile("", "dump_"+region+"_*.bin")
	if err != nil {
		s.Fatal("Failed creating temp file: ", err)
	}
	defer os.Remove(opTempFile.Name())

	var cmd *testexec.Cmd
	if len(region) > 0 {
		cmd = testexec.CommandContext(ctx, "flashrom", "-i", region, "-r", opTempFile.Name())
	} else { // full read is then assumed.
		cmd = testexec.CommandContext(ctx, "flashrom", "-r", opTempFile.Name())
	}
	if _, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	flashromElapsed := time.Since(flashromStart)

	return float64(flashromElapsed.Milliseconds())
}
