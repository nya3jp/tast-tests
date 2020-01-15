// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WPASanity,
		Desc:         "Verifies wpa_supplicant is up and running",
		Contacts:     []string{"deanliao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi"},
	})
}

func WPASanity(ctx context.Context, s *testing.State) {
	cmdOut, err := testexec.CommandContext(ctx, "sudo", "-u", "wpa", "-g", "wpa", "wpa_cli", "ping").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to ping wpa_supplicant")
	}

	output := string(cmdOut)
	if !strings.Contains(output, "PONG") {
		s.Fatal("Failed to see expected PONG reply")
	}
}
