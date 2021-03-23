// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VMConfig,
		Desc:         "Test that VM is configured correctly",
		Contacts:     []string{"hashimoto@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
		Timeout:      10 * time.Minute,
	})
}

func VMConfig(ctx context.Context, s *testing.State) {
	output, err := testexec.CommandContext(ctx, "nproc", "--all").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the number of host processors: ", err)
	}
	hostResult := bytes.TrimSpace(output)

	a := s.FixtValue().(*arc.PreData).ARC
	output, err = a.Command(ctx, "nproc", "--all").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the number of guest processors: ", err)
	}
	guestResult := bytes.TrimSpace(output)

	if !bytes.Equal(hostResult, guestResult) {
		s.Errorf("The number of processors are different: %s vs %s", hostResult, guestResult)
	}
}
