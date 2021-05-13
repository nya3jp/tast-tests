// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DiskErrors,
		Desc: "Checks disk error messages in dmesg",
		Contacts: []string{
			"nya@chromium.org",
			"tast-core@google.com",
		},
		Attr:    []string{"group:mainline", "informational"},
		Timeout: time.Minute,
	})
}

// diskErrorLinePatterns is a list of regexp patterns that match disk error
// messages.
var diskErrorLinePatterns = []*regexp.Regexp{
	// Buffer I/O error on dev loop9, logical block 0, async page read
	regexp.MustCompile(`Buffer I/O error on dev`),
	// print_req_error: I/O error, dev loop9, sector 0
	regexp.MustCompile(`print_req_error: I/O error`),
	// blk_update_request: I/O error, dev mmcblk0, sector 67964904
	regexp.MustCompile(`blk_update_request: I/O error`),
	// ata1.00: failed command: READ FPDMA QUEUED
	regexp.MustCompile(`failed command: .*FPDMA`),
	// ata1: hard resetting link
	regexp.MustCompile(`hard resetting link`),
}

func DiskErrors(ctx context.Context, s *testing.State) {
	// Save dmesg to an output file for inspection.
	f, err := os.Create(filepath.Join(s.OutDir(), "dmesg.txt"))
	if err != nil {
		s.Fatal("Failed to create output file: ", err)
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, "dmesg")
	cmd.Stdout = f
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run dmesg: ", err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		s.Fatal("Failed to seek: ", err)
	}

	// Look for error messages. Accumulate matched lines to a slice without
	// calling s.Error so that we can show the "Disk error found!" message
	// first.
	var errorLines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		for _, r := range diskErrorLinePatterns {
			if r.MatchString(line) {
				errorLines = append(errorLines, line)
				break
			}
		}
	}

	if err := sc.Err(); err != nil {
		s.Error("Encountered an error while scanning file: ", err)
	}

	if len(errorLines) > 0 {
		s.Error("Disk error found! Consider removing this DUT from pool")
		for _, line := range errorLines {
			s.Error(line)
		}
	}
}
