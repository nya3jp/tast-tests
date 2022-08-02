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
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdPowerdUpdateCheck,
		Desc: "Checks that the powerd plugin is enabled",
		Contacts: []string{
			"gpopoola@google.com",       // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"fwupd"},
		HardwareDeps: hwdep.D(
			hwdep.Battery(),               // Test doesn't run on ChromeOS devices without a battery.
			hwdep.ChromeEC(),              // Test requires Chrome EC to set battery to discharge via ectool.
			hwdep.SkipOnPlatform("celes"), // Platform does not register a discharge within timeout.
		),
		Timeout: fwupd.ChargingStateTimeout + 1*time.Minute,
		Params: []testing.Param{
			{
				Name: "ac_powerpresent",
				Val:  true,
			}, {
				Name: "no_acpower",
				Val:  false,
			}},
	})
}

// FwupdPowerdUpdateCheck sets the battery case, runs sequential update commands, and checks that they
// return with or without errors as appropriate.
func FwupdPowerdUpdateCheck(ctx context.Context, s *testing.State) {
	charge := s.Param().(bool)

	if !charge {
		defer fwupd.SetFwupdChargingState(ctx, !charge)
	}
	if err := fwupd.SetFwupdChargingState(ctx, charge); err != nil {
		s.Fatal("Failed to set charging state: ", err)
	}

	// This command runs an update on a fake device to see how fwupd behaves.
	upd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "install", "--allow-reinstall", "-v", fwupd.ReleaseURI)
	upd.Env = append(os.Environ(), "CACHE_DIRECTORY=/var/cache/fwupd")
	output, err := upd.Output(testexec.DumpLogOnError)
	if err != nil && charge {
		s.Errorf("%s failed erroneously: %v", shutil.EscapeSlice(upd.Args), err)
	} else if err == nil && !charge {
		s.Errorf("%s succeeded erroneously: %v", shutil.EscapeSlice(upd.Args), err)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output, 0644); err != nil {
		s.Error("Failed to write output from update: ", err)
	}
}
