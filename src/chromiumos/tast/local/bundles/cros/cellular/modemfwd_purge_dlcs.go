// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
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

	// modemfwd will uninstall the unused DLCs after 2 minutes.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for _, device := range manifest.Device {
			if device.GetDlc() != nil && device.GetDlc().GetDlcId() != "" {
				fsiPath := filepath.Join(dlc.FactoryInstallDir, device.GetDlc().GetDlcId())
				if _, err := os.Stat(fsiPath); !os.IsNotExist(err) {
					return errors.Wrapf(err, "directory %q was not deleted by dlcservice", fsiPath)
				}
			}
		}
		// Verify that there are no stale DLCs. This could happen if there is a modem
		// DLC ebuild, but the DlcId is not listed in the firmware_manifest.prototxt.
		files, err := filepath.Glob(filepath.Join(dlc.FactoryInstallDir, "/modem-fw-dlc-*"))
		if err != nil {
			return errors.Wrap(err, "failed to get contents of the DLC FSI directory")
		}
		if len(files) > 0 {
			return errors.Wrapf(err, "the following DLCs were not removed from the DLC FSI directory: %q", files)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20*time.Second + modemfwd.PurgeDlcsDelay}); err != nil {
		s.Fatal("Failed to unistall all DLCs: ", err)
	}

}
