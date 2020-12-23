// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BlockOutboundNetwork,
		Desc:         "Test the network blockage functionality of ARC++",
		Contacts:     []string{"bhansknecht@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func BlockOutboundNetwork(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	networkAvailable := func(ctx context.Context) bool {
		out, err := a.Command(ctx, "ping", "-c", "3", "-n", "-q", "8.8.8.8").Output(testexec.DumpLogOnError)
		// Ping is expected to return an error if it fails to ping the server.
		// Just log the error and return false.
		if err != nil {
			s.Log("Failed to run 'ping' command: ", err)
			return false
		}
		return strings.Contains(string(out), "3 received")
	}

	if !networkAvailable(ctx) {
		s.Fatal("Internet should be available at the start of the test")
	}

	if err := a.BlockOutbound(ctx); err != nil {
		s.Fatal("Failed to block ARC outbound traffic: ", err)
	}
	defer func() {
		if err := a.UnblockOutbound(cleanupCtx); err != nil {
			s.Fatal("Failed to unblock ARC outbound traffic: ", err)
		}
		if !networkAvailable(cleanupCtx) {
			s.Fatal("Internet should be available at the end of the test")
		}
	}()

	if networkAvailable(ctx) {
		s.Fatal("Internet should be unavailable when blocked")
	}

	if err := a.IsConnected(ctx); err != nil {
		s.Fatal("Failed to ensure ARC is still avialable through ADB: ", err)
	}

	if _, err := d.GetInfo(ctx); err != nil {
		s.Fatal("Failed to ensure UI Automator is available: ", err)
	}
}
