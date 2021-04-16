// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UreadaheadValidation,
		Desc: "Validates that ARC ureadahead pack exists and looks valid",
		Contacts: []string{"khmel@google.com",
			"alanding@google.com",
			"arc-performance@google.com"},
		Attr: []string{"group:mainline", "informational", "group:arc-functional"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 1 * time.Minute,
	})
}

func UreadaheadValidation(ctx context.Context, s *testing.State) {
	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		s.Fatal("Failed to get whether ARCVM is enabled: ", err)
	}

	packPath := ""
	if isVMEnabled {
		packPath = "/opt/google/vms/android/ureadahead.pack"
	} else {
		packPath = "/opt/google/containers/android/ureadahead.pack"
	}

	buf, err := testexec.CommandContext(ctx, "/sbin/ureadahead", "--dump", packPath).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("Failed to get the ureadahead stats %s, %v : ", packPath, err)
	}

	re := regexp.MustCompile(`\n(\d+) inode groups, (\d+) files, (\d+) blocks \((\d+) kB\)\n`)
	result := re.FindAllStringSubmatch(string(buf), -1)
	if len(result) != 1 {
		s.Fatal("Failed to parse ureadahead pack dump")
	}

	sizeKb, err := strconv.Atoi(result[0][4])
	if err != nil {
		s.Fatalf("Failed to parse %s, %v : ", result[0][4], err)
	}

	if sizeKb < 300*1024 {
		s.Errorf("Pack size %d kB is too small", sizeKb)
	}
}
