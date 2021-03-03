// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/testexec"
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
				deadline: 10000,
			},
		}, {
			Name: "fmap",
			Val: params{
				region:   "FMAP",
				deadline: 400,
			},
		}, {
			Name: "gbb",
			Val: params{
				region:   "GBB",
				deadline: 1700,
			},
		}, {
			Name: "rw_vpd",
			Val: params{
				region:   "RW_VPD",
				deadline: 1300,
			},
		}, {
			Name: "rw_elog",
			Val: params{
				region:   "RW_ELOG",
				deadline: 1300,
			},
		}, {
			Name: "rw_mrc_cache",
			Val: params{
				region:   "RW_MRC_CACHE",
				deadline: 1300,
			},
		}, {
			Name: "coreboot",
			Val: params{
				region:   "COREBOOT",
				deadline: 3000,
			},
		}},
	})
}

// FlashromPerf runs the flashrom utility to verify various expected behaviour
// is maintained.  The flashrom utility and times its total execution time for
// a complete probe and read of a given region of the SPI flash.
func FlashromPerf(ctx context.Context, s *testing.State) {
	var (
		readTime = perf.Metric{
			Name:      "flashrom_region_read_time",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}
	)
	p := s.Param().(params)

	perf := perf.NewValues()
	duration := testFlashromReadTime(ctx, s, p.region)
	perf.Set(readTime, duration)

	if err := perf.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
	if duration > p.deadline {
		s.Errorf("Flashrom took too long to execute, expected <= %v ms, got = %v ms", p.deadline, duration)
	}
}

func testFlashromReadTime(ctx context.Context, s *testing.State, region string) float64 {
	flashromStart := time.Now()

	opFileName := "/tmp/dump_" + region + ".bin"
	var cmd *testexec.Cmd
	if len(region) > 0 {
		cmd = testexec.CommandContext(ctx, "flashrom", "-i", region, "-r", opFileName)
	} else { // full read is then assumed.
		cmd = testexec.CommandContext(ctx, "flashrom", "-r", opFileName)
	}
	if _, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	flashromElapsed := time.Since(flashromStart)

	return float64(flashromElapsed.Milliseconds())
}
