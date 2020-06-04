// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/vmpstoredump"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VMPstoreDumpNoOptions,
		Desc:         "Test of vm_pstore_dump command",
		Contacts:     []string{"kimiyuki@google.com", "arcvm-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Pre:          arc.VMBooted(),
	})
}

func VMPstoreDumpNoOptions(ctx context.Context, s *testing.State) {
	vmPstoreDumpAfterBoot(ctx, s)
	vmPstoreDumpAfterLogRotation(ctx, s)
}

func vmPstoreDumpAfterBoot(ctx context.Context, s *testing.State) {
	// write a marker string to console output
	const markerString = "mv_pstore_dump-test-marker"
	if err := vmpstoredump.SendToKmsg(ctx, markerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}

	// run the vm_pstore_dump command
	buf, err := testexec.CommandContext(ctx, vmpstoredump.VMPstoreDumpPath).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the output of vm_pstore_dump command: ", err)
	}
	if !utf8.Valid(buf) {
		s.Fatal("The output is not a valid UTF-8 stiring")
	}
	out := string(buf)

	// check the output of the command
	if len(buf) > vmpstoredump.ConsoleBufferSize {
		s.Errorf("The output is too long. It must be less than or equal to the buffer size (%d): %d", vmpstoredump.ConsoleBufferSize, len(buf))
	}
	matched, err := regexp.MatchString(`^\[ *\d+\.\d+\] Linux version `, out)
	if err != nil || !matched {
		s.Error("Kernel's console output after booting should start with a string like \"[    0.000000] Linux version ...\" but it's not found in the result")
	}
	if !strings.Contains(out, markerString) {
		s.Error("The marker string we wrote to kernel's console output is not found in the output")
	}
}

func vmPstoreDumpAfterLogRotation(ctx context.Context, s *testing.State) {
	// write strings to console output
	const (
		headerMarkerString = "mv_pstore_dump-test-marker-first"
		footerMarkerString = "mv_pstore_dump-test-marker-last"
	)
	if err := vmpstoredump.SendToKmsg(ctx, headerMarkerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}
	if err := vmpstoredump.FillKmsg(ctx); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}
	if err := vmpstoredump.SendToKmsg(ctx, footerMarkerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}

	// run the vm_pstore_dump command
	buf, err := testexec.CommandContext(ctx, vmpstoredump.VMPstoreDumpPath).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the output of vm_pstore_dump command: ", err)
	}
	if !utf8.Valid(buf) {
		s.Fatal("The output is not a valid UTF-8 string")
	}
	out := string(buf)

	// check the output of the command
	if len(buf) < int(math.Floor(vmpstoredump.ConsoleBufferSize*0.99)) {
		s.Errorf("The output is too short. It's expected about the buffer size %d: %d", vmpstoredump.ConsoleBufferSize, len(buf))
	}
	if len(buf) > vmpstoredump.ConsoleBufferSize {
		s.Errorf("The output is too long. It must be less than or equal to the buffer size (%d): %d", vmpstoredump.ConsoleBufferSize, len(buf))
	}
	if strings.Contains(out, headerMarkerString) {
		s.Errorf("The marker string (%v) we wrote once to kernel's console output and cleared is found in the output", headerMarkerString)
	}
	if !strings.Contains(out, footerMarkerString) {
		s.Errorf("The marker string (%v) we wrote to kernel's console output is not found in the output", footerMarkerString)
	}
}
