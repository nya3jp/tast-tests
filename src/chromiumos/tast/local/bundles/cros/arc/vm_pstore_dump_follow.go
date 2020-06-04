// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"strings"
	"unicode/utf8"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/vmpstoredump"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VMPstoreDumpFollow,
		Desc:         "Test of vm_pstore_dump command",
		Contacts:     []string{"kimiyuki@google.com", "arcvm-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Pre:          arc.VMBooted(),
	})
}

func VMPstoreDumpFollow(ctx context.Context, s *testing.State) {
	// write a marker string to console output
	const firstMarkerString = "mv_pstore_dump-test-marker-first"
	if err := vmpstoredump.SendToKmsg(ctx, firstMarkerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}

	// start the vm_pstore_dump command
	cmd := testexec.CommandContext(ctx, vmpstoredump.VMPstoreDumpPath, "--follow")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start vm_pstore_dump command: ", err)
	}
	defer cmd.Kill()

	// write another strings to console output
	const (
		secondMarkerString = "mv_pstore_dump-test-marker-second"
		lastMarkerString   = "mv_pstore_dump-test-marker-last"
	)
	if err := vmpstoredump.SendToKmsg(ctx, secondMarkerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}
	if err := vmpstoredump.FillKmsg(ctx); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}
	if err := vmpstoredump.SendToKmsg(ctx, lastMarkerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}

	// stop the vm_pstore_dump command
	vmpstoredump.WaitPolling(ctx)
	if err := cmd.Kill(); err != nil {
		s.Error("Failed to kill vm_pstore_dump commnd: ", err)
	}
	buf := stdout.Bytes()
	if !utf8.Valid(buf) {
		s.Fatal("The output is not a valid UTF-8 string")
	}
	out := string(buf)

	// check the output of the command
	if len(buf) <= vmpstoredump.ConsoleBufferSize {
		s.Errorf("The output is too short. It must be at least the buffer size (%d): %d", vmpstoredump.ConsoleBufferSize, len(buf))
	}
	if !strings.Contains(out, firstMarkerString) {
		s.Errorf("The marker string (%v) we wrote to kernel's console output is not found in the output", firstMarkerString)
	}
	if !strings.Contains(out, secondMarkerString) {
		s.Errorf("The marker string (%v) we wrote to kernel's console output is not found in the output", secondMarkerString)
	}
	if !strings.Contains(out, lastMarkerString) {
		s.Errorf("The marker string (%v) we wrote to kernel's console output is not found in the output", lastMarkerString)
	}
}
