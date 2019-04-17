// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckMosys,
		Desc: "Checks the mosys command functionality",
		Contacts: []string{
			"derat@chromium.org",
			"kasaiah.bogineni@intel.com",
			"tast-users@chromium.org"},
	})
}

//CheckMosys verifies the defined mosys commands(in commands)
func CheckMosys(ctx context.Context, s *testing.State) {
	//Mosys sub-commands
	commands := []string{
		"mosys smbios info bios",
		"mosys ec info",
		"mosys platform name",
		"mosys eeprom map",
		"mosys platform vendor",
		"mosys eventlog list",
		"mosys memory spd print all",
	}

	for _, mosysCmd := range commands {
		s.Logf("Verifying the command '%s'", mosysCmd)
		cmd := testexec.CommandContext(ctx, "sh", "-c", mosysCmd)
		_, cmdErr := cmd.Output()
		if cmdErr != nil {
			cmd.DumpLog(ctx)
			s.Errorf("'%s' failed and the Error is '%v' ", mosysCmd, cmdErr)
		}
	}

}
