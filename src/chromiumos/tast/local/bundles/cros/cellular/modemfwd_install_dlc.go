// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	dlcp "chromiumos/system_api/dlcservice_proto"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/dlc"
	"chromiumos/tast/local/modemfwd"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModemfwdInstallDlc,
		Desc:         "Verifies that modemfwd installs the DLC on start",
		Contacts:     []string{"andrewlassalle@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_sim_active", "cellular_unstable"},
		Fixture:      "cellular",
		SoftwareDeps: []string{"modemfwd"},
		Timeout:      20 * time.Second,
	})
}

// ModemfwdInstallDlc Test
func ModemfwdInstallDlc(ctx context.Context, s *testing.State) {

	dlcID, err := cellular.GetDlcIDForVariant(ctx)
	if err != nil {
		s.Fatalf("Failed to get DLC ID: %s", err)
	}

	if dlcID == "" {
		return
	}

	if err := dlc.Purge(ctx, dlcID); err != nil {
		s.Fatalf("Failed to purge dlc %q: %s", dlcID, err)
	}

	defer func(ctx context.Context) {
		if err := upstart.StopJob(ctx, modemfwd.JobName); err != nil {
			s.Fatalf("Failed to stop %q: %s", modemfwd.JobName, err)
		}
		s.Log("modemfwd has stopped successfully")
	}(ctx)
	// modemfwd is initially stopped in the fixture SetUp
	if err := modemfwd.StartAndWaitForQuiescence(ctx); err != nil {
		s.Fatal("modemfwd failed during initialization: ", err)
	}
	s.Log("modemfwd has started successfully")

	state, err := dlc.GetDlcState(ctx, dlcID)
	if err != nil {
		s.Fatalf("Failed to get state info for DLC %q: %q", dlcID, err)
	}

	if dlcp.DlcState_State(state.State) != dlcp.DlcState_INSTALLED {
		s.Fatalf("Dlc was not installed. State is %q", dlcp.DlcState_State(state.State).String())
	}

}
