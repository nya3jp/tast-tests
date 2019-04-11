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
	const (
		path = "/opt/google/containers/android/rootfs/root/system/bin/run-as"
		key  = "security.capability"
		// security.capability with CAP_SETUID and CAP_SETGID encoded in hex.
		expect = "01000002c0000000000000000000000000000000"
	)

	out, err := testexec.CommandContext(ctx, "getfattr", "--only-values", "--name", key, path).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("Failed to get %s xattr for %s: %v", key, path, err)
	}
	if val := hex.EncodeToString(out); val != expect {
		s.Fatalf("Unexpected %s xattr for %s: got %s; want %s", key, path, val, expect)
	}
}
