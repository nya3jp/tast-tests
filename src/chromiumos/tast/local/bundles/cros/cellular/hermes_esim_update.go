// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HermesESIMUpdate,
		Desc: "Tests the eSIM update flow",
		Contacts: []string{
			"pholla@google.com",
			"chromeos-cellular-team@google.com",
		},
		Data: []string{
			"third_party/Test_multiscript_chrome_v1.xml",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_prod_esim"},
		Timeout: 3 * time.Minute,
	})
}

func HermesESIMUpdate(ctx context.Context, s *testing.State) {
	f, err := os.CreateTemp("", "invalid-update.xml")
	if err != nil {
		s.Fatal("Could not create invalid eOS update xml: ", err)
	}
	defer os.Remove(f.Name())

	performFWUpdate(ctx, s, f.Name(), false)
	performFWUpdate(ctx, s, s.DataPath("third_party/Test_multiscript_chrome_v1.xml"), true)
}

func performFWUpdate(ctx context.Context, s *testing.State, fwPath string, isValidUpdate bool) {
	s.Log("Updating ESIM FW using ", fwPath)
	if err := os.Chmod(fwPath, 0644); err != nil {
		s.Fatal("Unable to change permissions of eSIM fw: ", err)
	}
	s.Log("Restarting hermes")
	const hermesJobName = "hermes"
	if err := upstart.RestartJob(ctx, hermesJobName, upstart.WithArg("LOG_LEVEL", "-2"), upstart.WithArg("ESIM_FW_PATH", fwPath)); err != nil {
		s.Fatalf("Failed to restart job: %q, %s", hermesJobName, err)
	}
	euicc, _, err := hermes.WaitForEUICC(ctx, false)
	if err != nil {
		s.Fatal("Unable to get hermes euicc: ", err)
	}
	// The following Hermes call triggers an eOS update
	_, err = euicc.InstalledProfiles(ctx, true)
	if !isValidUpdate && err == nil {
		s.Fatal("eOS update should fail for invalid update")
	}
	if isValidUpdate && err != nil {
		s.Fatal("Failed to get installed profiles after eOS update: ", err)
	}
}
