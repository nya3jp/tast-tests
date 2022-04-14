// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"
	"os"

	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HermesESIMUpdate,
		Desc: "Iterates over profiles in an eUICC and enables them. At least 1 profile must be preinstalled",
		Contacts: []string{
			"pholla@google.com",
			"chromeos-cellular-team@google.com",
		},
		Data: []string{
			"third_party/Load_update_MFFXS-EUICC-GLC00.xml",
			"third_party/Rollback_update_MFFXS-EUICC-GLC00.xml",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_prod_esim"},
		Timeout: 3 * time.Minute,
	})
}

func HermesESIMUpdate(ctx context.Context, s *testing.State) {
	performFWUpdate(ctx, s, s.DataPath("third_party/Load_update_MFFXS-EUICC-GLC00.xml"))
	performFWUpdate(ctx, s, s.DataPath("third_party/Rollback_update_MFFXS-EUICC-GLC00.xml"))
}

func performFWUpdate(ctx context.Context, s *testing.State, fwPath string) {
	s.Log("Updating ESIM FW using ", fwPath )
	if err := os.Chmod(fwPath, 0644); err != nil {
		s.Fatal("Unable to change permissions of eSIM fw: ", err)
	}
	s.Log("Restarting hermes")
	const hermesJobName = "hermes"
	if err := upstart.RestartJob(ctx, hermesJobName, upstart.WithArg("LOG_LEVEL","-2"), upstart.WithArg("ESIM_FW_PATH",fwPath)); err != nil {
		s.Fatalf("Failed to restart job: %q, %s", hermesJobName, err)
	}
	euicc, _, err := hermes.WaitForEUICC(ctx, false)
	if err != nil {
		s.Fatal("Unable to get Hermes euicc: ", err)
	}
	_, err = euicc.InstalledProfiles(ctx, true)
	if err != nil {
		s.Fatal("Failed to get installed profiles: ", err)
	}
}
