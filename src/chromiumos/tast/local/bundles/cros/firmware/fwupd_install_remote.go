// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/firmware/fwupd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdInstallRemote,
		Desc: "Checks that fwupd can install using a remote repository",
		Contacts: []string{
			"campello@chromium.org",     // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"fwupd"},
		HardwareDeps: hwdep.D(
			hwdep.Battery(),  // Test doesn't run on ChromeOS devices without a battery.
			hwdep.ChromeEC(), // Test requires Chrome EC to set battery to charge via ectool.
		),
		Timeout: fwupd.ChargingStateTimeout + 1*time.Minute,
	})
}

// FwupdInstallRemote runs the fwupdtool utility and verifies that it
// can update a device in the system using a remote repository.
func FwupdInstallRemote(ctx context.Context, s *testing.State) {
	// make sure dut battery is charging/charged
	if err := fwupd.SetFwupdChargingState(ctx, true); err != nil {
		s.Fatal("Failed to set charging state: ", err)
	}

	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "install", "--allow-reinstall", "-v", fwupd.ReleaseURI)
	cmd.Env = append(os.Environ(), "CACHE_DIRECTORY=/var/cache/fwupd")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%q failed: %v", cmd.Args, err)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output, 0644); err != nil {
		s.Error("Failed to write output from update: ", err)
	}
}
