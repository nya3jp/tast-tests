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
	testCases := []struct {
		name string
		flag string
	}{
		{"CheckIdUser", "--user"},
		{"CheckIdGroup", "--group"},
		{"CheckIdGroups", "--groups"},
	}

	for _, tc := range testCases {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			cmd := testexec.CommandContext(ctx, "id", tc.flag, "chronos")
			expectedOut, err := cmd.Output()
			if err != nil {
				s.Fatalf("Failed to run 'id %s chronos': %v", tc.flag, err)
			}
			expectedOutStr := string(expectedOut)
			if strings.Count(expectedOutStr, "\n") != 1 {
				s.Fatalf("Unexpected 'id %s chronos' output: %q, wanted exactly 1 line",
					tc.flag, expectedOutStr)
			}
			expectedIds := strings.Split(strings.TrimSpace(expectedOutStr), " ")
			sort.Strings(expectedIds)
			expected := strings.Join(expectedIds, " ")

			cmd, err = testexec.CommandContextUser(ctx, "chronos", "id", tc.flag)
			if err != nil {
				s.Fatal("Failed to setup CommandContextUser: ", err)
			}
			testOut, err := cmd.Output()
			if err != nil {
				s.Fatalf("Failed to run 'id %s' as chronos: %v", tc.flag, err)
			}
			testOutStr := string(testOut)
			if strings.Count(testOutStr, "\n") != 1 {
				s.Fatalf("Unexpected 'id %s' output: %q, wanted exactly 1 line",
					tc.flag, testOutStr)
			}
			testIds := strings.Split(strings.TrimSpace(testOutStr), " ")
			sort.Strings(testIds)
			test := strings.Join(testIds, " ")

			if strings.Compare(expected, test) != 0 {
				s.Fatalf("Unexpected 'id %s' output: got %q, want %q",
					tc.flag, test, expected)
			}
		})
	}
}
