// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"unicode/utf8"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VMPstoreDump,
		Desc:         "Test of vm_pstore_dump command: check the kernel's console output after running vm_pstore_dump",
		Contacts:     []string{"kimiyuki@google.com", "arcvm-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm", "arc_pstore"},
		Fixture:      "arcBooted",
	})
}

// VMPstoreDump runs the vm_pstore_dump command and check whether it stops without something apparently wrong (e.g. segmentation fault). It's difficult to do more detailed tests because it's difficult to write strings to the console output when SELinux is enabled.
func VMPstoreDump(ctx context.Context, s *testing.State) {
	const (
		// vmPstoreDumpPath is the path to vm_pstore_dump command.
		vmPstoreDumpPath   = "/usr/bin/vm_pstore_dump"
		ramoopsConsoleSize = 0x40000
		// consoleBufferSize is the expected size of the ring buffer for the console output. The size is ramoops.console_size - persistent_ram_buffer.sig (4 byte) - persistent_ram_buffer.start (4 byte) - persistent_ram_buffer.size (4 byte)
		consoleBufferSize = ramoopsConsoleSize - 12
	)

	// run the vm_pstore_dump command
	buf, err := testexec.CommandContext(ctx, vmPstoreDumpPath).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the output of vm_pstore_dump command: ", err)
	}

	// check the output of the command
	if !utf8.Valid(buf) {
		s.Fatal("The output is not a valid UTF-8 stiring")
	}
	out := string(buf)
	if len(buf) > consoleBufferSize {
		s.Errorf("The output is too long. It must be less than or equal to the buffer size (%d): %d", consoleBufferSize, len(buf))
	}
	if matched, err := regexp.MatchString(`^\[ *\d+\.\d+\] Linux version `, out); err != nil {
		s.Error("Failed to check the output: ", err)
	} else if !matched {
		s.Error("Kernel's console output after booting should start with a string like \"[    0.000000] Linux version ...\" but it's not found in the result")
	}
}
