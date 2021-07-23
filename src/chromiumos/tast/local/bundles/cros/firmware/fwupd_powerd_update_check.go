// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type configs struct {
	discharge bool
}

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

//upsateChecker checks if an update is available and determines whether to update if there is or to reinstall one if there isn't
func updateChecker(ctx context.Context, s *testing.State) {

	cmd1 := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "get-updates")
	output1, err1 := cmd1.Output(testexec.DumpLogOnError)

	var cmd2 *testexec.Cmd

	if err1 == nil && output1 != nil {
		cmd2 = testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "update", "-v", "b585990a-003e-5270-89d5-3705a17f9a43", "--ignore-power")
		output2, err2 := cmd2.Output(testexec.DumpLogOnError)
		if err2 != nil {
			s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd1.Args), err1)
		}
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output2, 0644); err != nil {
			s.Error("Failed dumping fwupdmgr output: ", err)
		}
	} else {
		cmd2 = testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "reinstall", "-v", "b585990a-003e-5270-89d5-3705a17f9a43", "--ignore-power")
		output2, err2 := cmd2.Output(testexec.DumpLogOnError)
		if err2 != nil {
			s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd2.Args), err2)
		}
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output2, 0644); err != nil {
			s.Error("Failed dumping fwupdmgr output: ", err)
		}
	}
}

// FwupdPowerdUpdateCheck sets the battery case, runs sequential update commands, and checks that they
// return with or without errors as appropriate.
func FwupdPowerdUpdateCheck(ctx context.Context, s *testing.State) {

	set := s.Param().(configs)

	if set.discharge {
		setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 20)
		if err != nil {
			s.Fatalf("Failed to force battery to discharge: ", err)
		}
		time.Sleep(1200 * time.Second)
		defer setBatteryNormal(ctx)
	}

	updateChecker(ctx, s)

	cmd2 := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "reinstall", "-v", "b585990a-003e-5270-89d5-3705a17f9a43")
	output2, err2 := cmd2.Output(testexec.DumpLogOnError)
	if err2 == nil && set.discharge {
		s.Errorf("%s succeeded erroneously: %v", shutil.EscapeSlice(cmd2.Args), err2)
	} else if err2 != nil && !set.discharge {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd2.Args), err2)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr2.txt"), output2, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
	}

}
