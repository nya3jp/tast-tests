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

	p := perf.NewValues()
	duration = testFlashromReadTime()
	p.Set(readTime, duration)

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

func testFlashromReadTime() {
	flashromStart := time.Now()

	cmd := testexec.CommandContext(ctx, "flashrom", "-r", "dump.bin")
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	path := filepath.Join(s.OutDir(), "flashrom.txt")
	if err := ioutil.WriteFile(path, out, 0644); err != nil {
		s.Error("Failed to save flashrom output: ", err)
	}
	flashromElapsed = time.Since(flashromStart)

	// Return duration.
	return float64(flashromElapsed.Milliseconds())
}
