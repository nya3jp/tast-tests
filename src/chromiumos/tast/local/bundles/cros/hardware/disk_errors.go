// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
		Attr:    []string{"group:mainline"},
		Timeout: time.Minute,
	})
}

// diskErrorLinePatterns is a list of regexp patterns that match disk error
// messages.
var diskErrorLinePatterns = []*regexp.Regexp{
	// drivers/ata/libata-eh.c: ata_eh_link_report
	regexp.MustCompile(`exception Emask`),
	// drivers/ata/libata-eh.c: ata_eh_reset
	regexp.MustCompile(`hard resetting link`),
	// drivers/scsi/sd.c: sd_print_result
	regexp.MustCompile(`Result: hostbyte=.*driverbyte=`),
}

// diskErrorLines returns a subset of lines corresponding to disk error
// messages.
func diskErrorLines(lines []string) (matches []string) {
	for _, line := range lines {
		for _, r := range diskErrorLinePatterns {
			if r.MatchString(line) {
				matches = append(matches, line)
			}
		}
	}
	return matches
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

	b, err := ioutil.ReadFile(f.Name())
	if err != nil {
		s.Fatal("Failed to read dmesg.txt: ", err)
	}

	if errorLines := diskErrorLines(strings.Split(string(b), "\n")); len(errorLines) > 0 {
		s.Error("Disk error found! Consider removing this DUT from pool")
		for _, line := range errorLines {
			s.Error(line)
		}
	}
}
