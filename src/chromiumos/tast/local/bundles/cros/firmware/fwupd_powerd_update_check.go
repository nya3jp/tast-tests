// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/firmware/fwupd"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	// This is a string that appears when the computer is discharging.
	dischargeString = `uint32 [0-9]\s+uint32 2`
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdPowerdUpdateCheck, LacrosStatus: testing.LacrosVariantUnknown, Desc: "Checks that the powerd plugin is enabled",
		Contacts: []string{
			"gpopoola@google.com",       // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"fwupd"},
		HardwareDeps: hwdep.D(
			hwdep.Battery(),  // Test doesn't run on ChromeOS devices without a batttery.
			hwdep.ChromeEC(), // Test requires Chrome EC to set battery to discharge via ectool.
		),
		Params: []testing.Param{
			{
				Name: "no_acpower",
				Val:  true,
			}, {
				Name: "ac_powerpresent",
				Val:  false,
			}},
	})
}

// FwupdPowerdUpdateCheck sets the battery case, runs sequential update commands, and checks that they
// return with or without errors as appropriate.
func FwupdPowerdUpdateCheck(ctx context.Context, s *testing.State) {
	discharge := s.Param().(bool)

	if discharge {
		setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 20.0)
		if err != nil {
			s.Fatal("Failed to force battery to discharge: ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			cmd := testexec.CommandContext(ctx, "dbus-send", "--print-reply", "--system", "--type=method_call",
				"--dest=org.chromium.PowerManager", "/org/chromium/PowerManager",
				"org.chromium.PowerManager.GetBatteryState")
			output, err := cmd.Output(testexec.DumpLogOnError)
			if err != nil {
				return err
			}
			matched, err := regexp.Match(dischargeString, output)
			if !matched || err != nil {
				return errors.New("powerd has not registered a discharge")
			}
			return nil
		}, &testing.PollOptions{Timeout: 120 * time.Second}); err != nil {
			s.Fatalf("Battery polling was unsuccessful: %q", err)
		}

		defer setBatteryNormal(ctx)
	}

	// This command runs an update on a fake device to see how fwupd behaves.
	upd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "install", "--allow-reinstall", "-v", fwupd.ReleaseURI)
	upd.Env = append(os.Environ(), "CACHE_DIRECTORY=/var/cache/fwupd")
	output, err := upd.Output(testexec.DumpLogOnError)
	if err == nil && discharge {
		s.Errorf("%s succeeded erroneously: %v", shutil.EscapeSlice(upd.Args), err)
	} else if err != nil && !discharge {
		s.Errorf("%s failed erroneously: %v", shutil.EscapeSlice(upd.Args), err)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output, 0644); err != nil {
		s.Error("Failed to write output from update: ", err)
	}
}
