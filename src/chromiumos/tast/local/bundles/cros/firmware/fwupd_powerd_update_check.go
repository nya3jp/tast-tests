// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type configs struct {
	targetMssg string
	targetMssg2 string
	trigger    bool
}

//const (
//	acPowerError = `failed to update_prepare using powerd: Cannot install update without external power unless forced`
//	batteryThresholdError = `Cannot install update when system battery is not at least 10% unless forced`
//	updatePermitted = `FuEngine%s{1,}Emitting%sPropertyChanged('Status'='device-write')%s{1,}Writing?`
//)

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
			Val: configs{
			},
		}, {
			Name: "no_acpower",
			Val: configs{
				targetMssg: `Successfully installed firmware`,
				targetMssg2: `ignoring Integrated Webcam? with status idle`,
				trigger:    true,
			},
		}, {
			Name: "ac_powerpresent",
			Val: configs{
				targetMssg: `Successfully installed firmware`,
				targetMssg2: `Successfully installed firmware`,
				trigger:    false,
			},
		}},
	})
}

// checkForUpdateMssg verifies that powerd was found among enabled plugins */
func checkForUpdateMssg(output []byte, goalString string) error {
	matched, err := regexp.Match(goalString, output)
	if err != nil {
		return err
	}
	if !matched {
		return errors.New("powerd did not handle batery case correctly")
	}
	return nil
}

// FwupdPowerdUpdateCheck runs a given update command, retrieves the output, and
// checks that the update was permitted/blocked appropriately
func FwupdPowerdUpdateCheck(ctx context.Context, s *testing.State) {

	set := s.Param().(configs)

	if set.trigger {
		setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 20)
		if err != nil {
			s.Fatalf("Failed to force battery to discharge: ", err)
		}
		defer setBatteryNormal(ctx)
	}


	//Debugging Discharge
	cmdb := testexec.CommandContext(ctx, "cat", "/sys/class/power_supply/BAT0/status")
	outputb, errb := cmdb.Output(testexec.DumpLogOnError)
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgrb.txt"), outputb, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
	}
	s.Log("%q", outputb)
	if errb != nil {
		s.Log(errb)
	}


	cmd1 := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "update", "-v", "b585990a-003e-5270-89d5-3705a17f9a43", "--ignore-power")
	output1, err1 := cmd1.Output(testexec.DumpLogOnError)
	if err1 != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd1.Args), err1)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output1, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
	}
	if err := checkForUpdateMssg(output1, set.targetMssg); err != nil {
		s.Error("message not found: ", err)
	}


	cmd2 := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "reinstall", "-v", "b585990a-003e-5270-89d5-3705a17f9a43")
	output2, err2 := cmd2.Output(testexec.DumpLogOnError)
	if err2 != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd2.Args), err2)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr2.txt"), output2, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
	}

	if err := checkForUpdateMssg(output2, set.targetMssg2); err != nil {
	        s.Error("message not found: ", err)
	}

}
