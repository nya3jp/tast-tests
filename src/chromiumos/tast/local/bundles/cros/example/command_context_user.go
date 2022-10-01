// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"sort"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CommandContextUser,
		Desc: "Example and test of the CommandContextUser function",
		Contacts: []string{
			"hesling@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// CommandContextUser exercises CommandContextUser function.
//
// We ensure that the uid, gid, and group ids are the same when running
// "id" as the user as it is running "id" with the user specified.
// We can't just run "id" and "id chronos", since it includes a security
// context field that might differ.
func CommandContextUser(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx,
		"bash", "-c",
		"id --user chronos && id --group chronos && id --groups chronos",
	)
	expectedOut, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to run id: ", err)
	}

	cmd, err = testexec.CommandContextUser(ctx, "chronos",
		"bash", "-c",
		"id --user && id --group && id --groups",
	)
	if err != nil {
		s.Fatal("Failed to setup CommandContextUser: ", err)
	}
	testOut, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to run id as chronos: ", err)
	}

	expectedOutLines := strings.Split(string(expectedOut), "\n")
	testOutLines := strings.Split(string(testOut), "\n")

	// We only have 3 lines with 3 \n's, but the final \n creates a 4th
	// empty split item.
	if count := len(expectedOutLines); count != 4 {
		s.Fatalf("The expected output has %d newlines, but we expected 4", count)
	}
	if count := len(testOutLines); count != 4 {
		s.Fatalf("The test output has %d newlines, but we expected 4", count)
	}
	// Check that the list of IDs in each of the 3 lines match, when sorted.
	// Only the third, last, line should actually contain a list of IDs.
	for line := 0; line < 3; line++ {
		expectedIds := strings.Split(expectedOutLines[line], " ")
		sort.Strings(expectedIds)
		expectedStr := strings.Join(expectedIds, " ")

		testIds := strings.Split(testOutLines[line], " ")
		sort.Strings(testIds)
		testStr := strings.Join(testIds, " ")

		if strings.Compare(expectedStr, testStr) != 0 {
			s.Fatalf("ID mismatch of output on line %d: got %q, want %q",
				line, testStr, expectedStr)
		}
	}
}
