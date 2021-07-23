// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type configs struct {
	discharge bool
}

const (
	dischargeString = `uint32 [0-9]\s+uint32 2`
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
			//test doesn't run on ChromeOS devices without a batttery
			hwdep.Battery(),
		),
		Params: []testing.Param{{
			Val: configs{},
		}, {
			Name: "no_acpower",
			Val: configs{
				discharge: true,
			},
		}, {
			Name: "ac_powerpresent",
			Val: configs{
				discharge: false,
			},
		}},
	})
}

// FwupdPowerdUpdateCheck sets the battery case, runs sequential update commands, and checks that they
// return with or without errors as appropriate.
func FwupdPowerdUpdateCheck(ctx context.Context, s *testing.State) {

	var upd *testexec.Cmd
	set := s.Param().(configs)

	if err := upstart.RestartJob(ctx, "fwupd"); err != nil {
		s.Error("Failed to restart fwupd: ", err)
	}

	if set.discharge {
		setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 20)
		if err != nil {
			s.Fatalf("Failed to force battery to discharge: %q", err)
		}
		for {
			cmd := testexec.CommandContext(ctx, "dbus-send", "--print-reply", "--system", "--type=method_call",
				"--dest=org.chromium.PowerManager", "/org/chromium/PowerManager",
				"org.chromium.PowerManager.GetBatteryState")

			output, err := cmd.Output(testexec.DumpLogOnError)
			if err != nil {
				s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
			}

			if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgrcall.txt"), output, 0644); err != nil {
				s.Error("Failed dumping fwupdmgr output: ", err)
			}

			matched, err := regexp.Match(dischargeString, output)
			if err != nil {
				s.Error("powerd has not registered a discharge: ", err)
			}
			if matched {
				break
			}
		}
		// sleep timers allow computer to readjust to altered battery states
		testing.Sleep(ctx, 10*time.Second)
		defer testing.Sleep(ctx, 5*time.Second)
		defer setBatteryNormal(ctx)
	}

	upd = testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "update", "-v", "b585990a-003e-5270-89d5-3705a17f9a43")
	output, err := upd.Output(testexec.DumpLogOnError)
	if err == nil && set.discharge {
		s.Errorf("%s succeeded erroneously: %v", shutil.EscapeSlice(upd.Args), err)
	} else if err != nil && !set.discharge {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(upd.Args), err)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr2.txt"), output, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
	}
}
