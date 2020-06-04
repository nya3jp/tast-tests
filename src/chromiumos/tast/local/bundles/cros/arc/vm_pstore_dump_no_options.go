// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
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
		Params: []testing.Param{{
			Name: "no_options",
			Val:  false,
		}, {
			Name: "use_follow",
			Val:  true,
		}},
	})
}

// VMPstoreDumpNoOptions runs the vm_pstore_dump command and check whether it stops without something apparently  wrong (e.g. segmentation fault). It's difficult to do more detailed tests because it's difficult to write strings to the console output when SELinux is enabled.
func VMPstoreDumpNoOptions(ctx context.Context, s *testing.State) {
	// run the vm_pstore_dump command
	buf, err := testexec.CommandContext(ctx, vmpstoredump.VMPstoreDumpPath).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the output of vm_pstore_dump command: ", err)
	}

	// check the output of the command
	if !utf8.Valid(buf) {
		s.Fatal("The output is not a valid UTF-8 stiring")
	}
	out := string(buf)
	if len(buf) > vmpstoredump.ConsoleBufferSize {
		s.Errorf("The output is too long. It must be less than or equal to the buffer size (%d): %d", vmpstoredump.ConsoleBufferSize, len(buf))
	}
	matched, err := regexp.MatchString(`^\[ *\d+\.\d+\] Linux version `, out)
	if err != nil || !matched {
		s.Error("Kernel's console output after booting should start with a string like \"[    0.000000] Linux version ...\" but it's not found in the result")
	}
}
