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
		Func:         GpuSandboxed,
		Desc:         "Verify GPU sandbox status",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func GpuSandboxed(ctx context.Context, s *testing.State) {
	const (
		url      = "chrome://gpu"
		waitExpr = "browserBridge.isSandboxedForTesting()"
	)

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// "chrome://about" cannot be opened directly, due to cdp
	// implementation.
	// https://github.com/mafredri/cdp/blob/master/devtool/devtool.go#L80
	// This is workaround; create the connection and then naviagate
	// to the page.
	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to create a new connection: ", err)
	}
	defer conn.Close()

	if err = conn.Navigate(ctx, url); err != nil {
		s.Fatalf("Failed to open %q: %v", url, err)
	}

	{
		ectx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err = conn.WaitForExpr(ectx, waitExpr); err != nil {
			s.Fatalf("Failed to evaluate %q in %s", waitExpr, url)
		}
	}
}
