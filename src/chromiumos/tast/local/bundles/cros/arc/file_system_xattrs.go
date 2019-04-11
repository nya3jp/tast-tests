// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/hex"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FileSystemXattrs,
		Desc: "Verifies filesystem extended attributes for ARC container",
		Contacts: []string{
			"kroot@chromium.org", // Original author.
			"arc-core@google.com",
			"hidehiko@chromium.org", // Tast port author.
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func FileSystemXattrs(ctx context.Context, s *testing.State) {
	runTest := func(path, key, expect string) {
		out, err := testexec.CommandContext(ctx, "getfattr", "--only-values", "--name", key, path).Output(testexec.DumpLogOnError)
		if err != nil {
			s.Errorf("Failed to take the %s xattr for %s: %v", key, path, err)
			return
		}
		val := hex.EncodeToString(out)
		if val != expect {
			s.Errorf("Unexpected %s xattr for %s: got %s; want %s", key, path, val, expect)
		}
	}

	runTest("/opt/google/containers/android/rootfs/root/system/bin/run-as",
		"security.capability",
		// security.capability with CAP_SETUID and CAP_SETGID encoded in hex
		"01000002c0000000000000000000000000000000")
}
