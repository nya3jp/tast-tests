// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"unicode"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FlexID,
		Desc: "Tests that the go/chromeos-flex-id is properly-formed",
		Contacts: []string{
			"josephsussman@google.com", // Test author
			"chromeos-flex-eng@google.com",
		},
		SoftwareDeps: []string{"flex_id"},
		Attr:         []string{"group:mainline"},
	})
}

// isAllASCII returns true if output only contains ASCII characters.
func isAllASCII(output string) bool {
	for i := 0; i < len(output); i++ {
		if output[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func FlexID(ctx context.Context, s *testing.State) {
	out, err := testexec.CommandContext(ctx, "flex_id_tool").CombinedOutput()
	status, ok := testexec.ExitCode(err)

	outString := string(out)
	if !ok {
		s.Fatalf("flex_id_tool exited with status %d: %s", status, outString)
	}

	if !isAllASCII(outString) {
		s.Errorf("%s contains non-ASCII characters", outString)
	}
}
