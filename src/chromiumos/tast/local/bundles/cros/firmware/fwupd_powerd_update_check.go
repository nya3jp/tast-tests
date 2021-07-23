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

type params struct {
	targetMssg string
	trigger bool
	flags string
}

//const (
//	acPowerError = `Cannot install update without external power unless forced`
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
		Params: []testing.Params{{
			Val: params{
				//targetMssg:
				//variableB=
			},
		},{
			Name: "No_AC_Power",
			Val: params{
				targetMssg:`Cannot install update without external power unless forced`,
				trigger := "TRUE",
				flags  := nil,
			},
		},{
			Name: "AC_Power_Present"
			Val: params{
				targetMssg:`FuEngine\s+Emitting\sPropertyChanged('Status'='device-write')\s+Writing?`,
				trigger := "FALSE",
				flags := nil,
			},
		},{
			Name: "Power_Ignored"
			Val: params{
				targetMssg: `FuEngine\s+Emitting\sPropertyChanged('Status'='device-write')\s+Writing?`,
				trigger := "FALSE",
				flags := "--ignore-power"
			},

		}},

	})
}
func disableACPower (ctx context.Context, s *testing.State) {
	setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 40)
	if err != nil{
		s.Fatalf("Failed to force battery to discharge: ", err)
	}
}
// checkForUpdateMssg verifies that powerd was found among enabled plugins */
func checkForUpdateMssg(output []byte) error {
	matched, err := regexp.Match(targetMssg, output)
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
	if trigger {
		disableACPower(ctx, s)
	}

	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "update", "-v", "b585990a-003e-5270-89d5-3705a17f9a43", flags)

	output, err := cmd.Output(testexec.DumpLogOnError)

	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output, 0644); err != nil {
		s.Fatal("Failed dumping fwupdmgr output: ", err)
	}

	if err := checkForUpdateMssg(output); err != nil {
		s.Fatal("message not found: ", err)
	}
}
