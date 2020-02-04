// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeTimezoneInfo,
		Desc: "Check that we can probe cros_healthd for timezone info",
		Contacts: []string{
			"jschettler@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeTimezoneInfo(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		s.Fatal("Failed to start cros_healthd: ", err)
	}

	b, err := testexec.CommandContext(ctx, "telem", "--category=timezone").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run 'telem --category=timezone': ", err)
	}

	// Log output to file for debugging.
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "command_output.txt"), b, 0644); err != nil {
		s.Errorf("Failed to write output to %s: %v", filepath.Join(s.OutDir(), "command_output.txt"), err)
	}

	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) != 2 {
		s.Fatalf("Wrong number of lines: got %v, want 2", len(lines))
	}

	// Verify the headers are correct.
	want := []string{"posix_timezone", "timezone_region"}
	got := strings.Split(lines[0], ",")
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify the reported timezone info.
	vals := strings.Split(lines[1], ",")
	if vals[0] == "" {
		s.Error("Missing posix_timezone")
	}

	if vals[1] == "" {
		s.Error("Missing timezone_region")
	}
}
