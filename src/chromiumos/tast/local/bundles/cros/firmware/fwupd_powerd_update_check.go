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
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type configs struct {
	discharge bool
}

const (
	dischargeString = "uint32 2"
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

//updateChecker checks if an update is available and determines whether to update if there is or to reinstall one if there isn't
func updateChecker(ctx context.Context) string {

	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "get-updates")
	err := cmd.Run(testexec.DumpLogOnError)

	if err == nil {
		return "update"
	} else {
		return "reinstall"
	}
}

// FwupdPowerdUpdateCheck sets the battery case, runs sequential update commands, and checks that they
// return with or without errors as appropriate.
func FwupdPowerdUpdateCheck(ctx context.Context, s *testing.State) {

	var upd1 *testexec.Cmd
	set := s.Param().(configs)

	if set.discharge {
		setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 20)
		if err != nil {
			s.Fatalf("Failed to force battery to discharge: ", err)
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
			if matched {
				break
			}
		}
		// sleep timers allow computer to readjust to altered battery states
		time.Sleep(10 * time.Second)
		defer time.Sleep(5 * time.Second)
		defer setBatteryNormal(ctx)
	}

	subcmd := updateChecker(ctx)
	upd1 = testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", subcmd, "-v", "b585990a-003e-5270-89d5-3705a17f9a43", "--ignore-power")
	output1, err1 := upd1.Output(testexec.DumpLogOnError)
	if err1 != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(upd1.Args), err1)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output1, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
		}

	upd2 := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "reinstall", "-v", "b585990a-003e-5270-89d5-3705a17f9a43")
	output2, err2 := upd2.Output(testexec.DumpLogOnError)
	if err2 == nil && set.discharge {
		s.Errorf("%s succeeded erroneously: %v", shutil.EscapeSlice(upd2.Args), err2)
	} else if err2 != nil && !set.discharge {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(upd2.Args), err2)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr2.txt"), output2, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
	}

}
