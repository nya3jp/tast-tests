// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/dlc"
	"chromiumos/tast/local/modemfwd"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModemfwdPurgeDlcs,
		Desc:         "Verifies that modemfwd removes all the unused modem DLCs",
		Contacts:     []string{"andrewlassalle@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_sim_active", "cellular_unstable"},
		Fixture:      "cellular",
		SoftwareDeps: []string{"modemfwd"},
		Timeout:      1*time.Minute + modemfwd.PurgeDlcsDelay,
	})
}

// ModemfwdPurgeDlcs Test
func ModemfwdPurgeDlcs(ctx context.Context, s *testing.State) {
	startTime := time.Now()

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

	manifest, err := cellular.ParseModemFirmwareManifest(ctx)
	if err != nil {
		s.Fatal("Failed to parse the firmware manifest: ", err)
	}

	// modemfwd will unistall the unused DLCs after 2 minutes.
	// Sleep for a little over 2 minutes to give it enough time to delete the DLCs.
	sleepTime := 20*time.Second + modemfwd.PurgeDlcsDelay - time.Now().Sub(startTime)
	s.Log("Sleeping for ", sleepTime)
	testing.Sleep(ctx, sleepTime)

	for _, device := range manifest.Device {
		if device.GetDlcId() != "" {
			fsiPath := filepath.Join(dlc.FactoryInstallDir, device.GetDlcId())
			if _, err := os.Stat(fsiPath); !os.IsNotExist(err) {
				s.Fatalf("Directory %q was not deleted by dlcservice: %s", fsiPath, err)
			}
		}
	}

	// Verify that there are no stale DLCs. This could happen if there is a modem
	// DLC ebuild, but the DlcId is not listed in the firmware_manifest.prototxt.
	files, err := filepath.Glob(filepath.Join(dlc.FactoryInstallDir, "/modem-fw-dlc-*"))
	if err != nil {
		s.Fatal("Failed to get contents of the DLC FSI directory: ", err)
	}
	if len(files) > 0 {
		s.Fatalf("The following DLCs were not removed from the DLC FSI directory: %q", files)
	}
}
