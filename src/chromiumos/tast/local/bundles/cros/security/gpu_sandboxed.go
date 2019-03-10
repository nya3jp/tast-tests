// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GPUSandboxed,
		Desc: "Verify GPU sandbox status",
		Contacts: []string{
			"jorgelo@chromium.org",  // Security team
			"hidehiko@chromium.org", // Tast port author
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome_login", "gpu_sandboxing"},
		Pre:          chrome.LoggedIn(),
	})
}

func GPUSandboxed(ctx context.Context, s *testing.State) {
	const (
		url      = "chrome://gpu"
		waitExpr = "browserBridge.isSandboxedForTesting()"
	)

	cr := s.PreValue().(*chrome.Chrome)
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to create a new connection: ", err)
	}
	defer conn.Close()

	ectx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = conn.WaitForExpr(ectx, waitExpr); err != nil {
		s.Fatalf("Failed to evaluate %q in %s", waitExpr, url)
	}
}
