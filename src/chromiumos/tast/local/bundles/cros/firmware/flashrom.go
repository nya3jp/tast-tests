// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type params struct {
	region string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Flashrom,
		Desc: "Checks that flashrom can find a SPI ROM",
		Contacts: []string{
			"kmshelton@chromium.org",       // Test Author
			"quasisec@chromium.org",        // CrOS Flashrom Maintainer
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:mainline", "group:labqual"},
		SoftwareDeps: []string{"flashrom"},
	})
	testing.AddTest(&testing.Test{
		Func: FlashromPerf,
		Desc: "Flashrom perf metrics",
		Contacts: []string{
			"kmshelton@chromium.org",       // Test Author
			"quasisec@chromium.org",        // CrOS Flashrom Maintainer
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:mainline", "group:labqual"},
		SoftwareDeps: []string{"flashrom"},
		Params: []testing.Param{{
			Name: "fmap",
			Val: params{
				region: "FMAP",
			},
		}, {
			Name: "coreboot",
			Val: params{
				region: "COREBOOT",
			},
		}},
	})
}

// Flashrom runs the flashrom utility and confirms that flashrom was able to
// communicate with a SPI flash.
func Flashrom(ctx context.Context, s *testing.State) {
	// This test intentionally avoids SPI ROM read and write operations, so as not
	// to stress devices-under-test.
	cmd := testexec.CommandContext(ctx, "flashrom", "--verbose")
	re := regexp.MustCompile(`Found .* flash chip`)
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	} else if outs := string(out); !re.MatchString(outs) {
		path := filepath.Join(s.OutDir(), "flashrom.txt")
		if err := ioutil.WriteFile(path, out, 0644); err != nil {
			s.Error("Failed to save flashrom output: ", err)
		}
		s.Fatalf("Failed to confirm flashrom could find a flash chip.  "+
			"Output of %q did not contain %q (saved output to %s).",
			shutil.EscapeSlice(cmd.Args), re, filepath.Base(path))
	}
}

// FlashromPerf runs the flashrom utility and times its total execution time for
// a complete probe and read of the SPI flash.
func FlashromPerf(ctx context.Context, s *testing.State) {
	var (
		readTime = perf.Metric{
			Name:      "flashrom_read_time",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}
	)

	// TODO split up timing per region duration.
	for _, p = range s.Param().(params) {
		perf := perf.NewValues()
		duration := testFlashromReadTime(ctx, s, p.region)
		perf.Set(readTime, duration)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

func testFlashromReadTime(ctx context.Context, s *testing.State, region string) float64 {
	flashromStart := time.Now()

	opFileName = "dump_" + r + ".bin"
	cmd := testexec.CommandContext(ctx, "flashrom", "-i", region, "-r", opFileName)
	if _, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	flashromElapsed := time.Since(flashromStart)

	// Return duration.
	return float64(flashromElapsed.Milliseconds())
}
