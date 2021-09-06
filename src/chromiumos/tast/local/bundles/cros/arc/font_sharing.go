// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FontSharing,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test that font-sharing from Chrome OS to ARC works",
		Contacts:     []string{"hashimoto@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
		Timeout:      10 * time.Minute,
	})
}

func FontSharing(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	output, err := a.Command(ctx, "ls", "-sZL", "/system/fonts").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the font file info: ", err)
	}

	// The output looks like this:
	//  total 74979
	//    106 u:object_r:system_file:s0   /system/fonts/DroidSansMono.ttf
	//  10192 u:object_r:cros_usr_dirs:s0 /system/fonts/NotoColorEmoji.ttf
	//  ...
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.Fields(line)
		if tokens[0] == "total" { // Skip the first line.
			continue
		}

		size, err := strconv.ParseInt(tokens[0], 10, 64)
		if err != nil {
			s.Fatal("Failed to parse size: ", line)
		}

		// Font-sharing creates empty placeholder files, but they are used as mountpoints for bind-mounts.
		// If we are seeing empty files, that means bind-mount is not working well.
		if size == 0 {
			s.Fatal("Empty font file detected: ", line)
		}

		// Font-sharing should be applied to large font files.
		// These files must originate from Chrome OS.
		const fontSharingThresholdKB = 1024
		securityContext := tokens[1]
		if size >= fontSharingThresholdKB &&
			securityContext != "u:object_r:cros_usr_dirs:s0" {
			s.Fatal("Large font file with non-Chrome OS security context: ", line)
		}
	}
}
