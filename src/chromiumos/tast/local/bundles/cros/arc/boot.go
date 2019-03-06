// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Boot,
		Desc:     "Checks that Android boots",
		Contacts: []string{"nya@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{
			"android_all", // Run on master-arc-dev, too.
			"chrome_login",
		},
		Timeout: 4 * time.Minute,
	})
}

func Boot(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Run "pm list packages" and ensure "android" package exists.
	// This ensures package manager service is running at least.
	cmd := a.Command(ctx, "pm", "list", "packages")
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("pm list failed: ", err)
	}

	pkgs := strings.Split(string(out), "\n")
	found := false
	for _, p := range pkgs {
		if p == "package:android" {
			found = true
			break
		}
	}

	if !found {
		s.Error("android package not found: ", pkgs)
	}
}
