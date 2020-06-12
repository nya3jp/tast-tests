// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"reflect"

	"chromiumos/tast/local/croshealthd"
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
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeTimezoneInfo(ctx context.Context, s *testing.State) {
	records, err := croshealthd.RunAndParseTelem(ctx, croshealthd.TelemCategoryTimezone, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get timezone telemetry info: ", err)
	}

	if len(records) != 2 {
		s.Fatalf("Wrong number of output lines: got %d; want 2", len(records))
	}

	// Verify the headers are correct.
	want := []string{"posix_timezone", "timezone_region"}
	got := records[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify the reported timezone info.
	vals := records[1]
	if vals[0] == "" {
		s.Error("Missing posix_timezone")
	}

	if vals[1] == "" {
		s.Error("Missing timezone_region")
	}
}
