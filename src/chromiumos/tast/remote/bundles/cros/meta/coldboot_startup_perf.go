// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"path/filepath"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ColdbootStartupPerf,
		Desc:     "Captures startup coldboot metrics for Lacros",
		Contacts: []string{"hidehiko@chromium.org", "tvignatti@igalia.com", "lacros-team@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func ColdbootStartupPerf(ctx context.Context, s *testing.State) {
	// TODO(tvignatti): Find a way to get rid of CTRL+D trick "OS verification is OFF" every time it boots

	// TODO(tvignatti): Add iteration runs

	d := s.DUT()

	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Need to reconnect to the gRPC server after rebooting DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	s.Run(ctx, "BrowserLaunch", func(ctx context.Context, s *testing.State) {
		resultsDir := filepath.Join(s.OutDir(), "subtest_results")
		flags := []string{
			"-resultsdir=" + resultsDir,
		}

		_, _, err := tastrun.Exec(ctx, s, "run", flags, []string{"lacros.StartupPerf.rootfs_primary"})
		if err != nil {
			s.Fatal("Failed to run tast: ", err)
		}
	})
}
