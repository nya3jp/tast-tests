// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VMConfig,
		Desc:         "Test that VM is configured correctly",
		Contacts:     []string{"hashimoto@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      10 * time.Minute,
	})
}

func VMConfig(ctx context.Context, s *testing.State) {
	// Shorten the context to save time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(cleanupCtx); err != nil {
			s.Fatal("Failed to close Chrome: ", err)
		}
	}()
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer func() {
		if err := a.Close(); err != nil {
			s.Fatal("Failed to close ARC connection: ", err)
		}
	}()

	output, err := testexec.CommandContext(ctx, "nproc", "--all").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the number of host processors: ", err)
	}
	hostResult := strings.TrimSpace(string(output))

	output, err = a.Command(ctx, "nproc", "--all").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the number of guest processors: ", err)
	}
	guestResult := strings.TrimSpace(string(output))

	if hostResult != guestResult {
		s.Errorf("The number of processors are different: %s vs %s", hostResult, guestResult)
	}
}
