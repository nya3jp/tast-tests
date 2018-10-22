// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Boot,
		Desc:         "Checks that Android boots",
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Ready(),
	})
}

func Boot(ctx context.Context, s *testing.State) {
	a := arc.Get(s)

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
