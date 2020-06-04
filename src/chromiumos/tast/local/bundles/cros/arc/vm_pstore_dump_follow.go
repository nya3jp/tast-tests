// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"regexp"
	"time"
	"unicode/utf8"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/vmpstoredump"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VMPstoreDumpFollow,
		Desc:         "Test of vm_pstore_dump --follow command",
		Contacts:     []string{"kimiyuki@google.com", "arcvm-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Pre:          arc.VMBooted(),
	})
}

func VMPstoreDumpFollow(ctx context.Context, s *testing.State) {
	// start the vm_pstore_dump command
	cmd := testexec.CommandContext(ctx, vmpstoredump.VMPstoreDumpPath, "--follow")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start vm_pstore_dump command: ", err)
	}
	defer cmd.Kill()

	// wait for a while for the polling about the ring buffer
	testing.Sleep(ctx, 5*time.Second)

	// stop the vm_pstore_dump command
	if err := cmd.Kill(); err != nil {
		s.Error("Failed to kill vm_pstore_dump commnd: ", err)
	}
	buf := stdout.Bytes()

	// check the output of the command
	if !utf8.Valid(buf) {
		s.Fatal("The output is not a valid UTF-8 string")
	}
	out := string(buf)
	matched, err := regexp.MatchString(`^\[ *\d+\.\d+\] Linux version `, out)
	if err != nil || !matched {
		s.Error("Kernel's console output after booting should start with a string like \"[    0.000000] Linux version ...\" but it's not found in the result")
	}
}
