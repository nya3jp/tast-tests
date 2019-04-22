// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"regexp"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckCrossystem,
		Desc: "Checks that the crossystem command basic functionality is present",
		Contacts: []string{
			"semenzato@chromium.org",     // Autotest author
			"kasaiah.bogineni@intel.com", // Port author
			"tast-users@chromium.org"},
		Attr: []string{"informational"},
	})
}

// CheckCrossystem checks that the "crossystem" command basic functionality is present.
// This includes commands that rely on the presence and correct
// initialization of the chromeos driver (drivers/platform/chromeos.c)
// in the kernel
func CheckCrossystem(ctx context.Context, s *testing.State) {
	crossystemCmd := "crossystem"
	alphaNum := `^[\d\w]+$`
	num := `^[\d]+$`
	hexNum := `^0x[\da-fA-F]+$`
	bit := `^[01]$`
	anything := `^[^(error)]` // match everything that isn't an error and empty
	cmdRegexMap := map[string]string{
		"cros_debug":  bit,
		"debug_build": bit,
		"devsw_boot":  bit,
		"devsw_cur":   bit,
		"fwid":        anything,
		"hwid":        anything,
		"loc_idx":     num,
		"mainfw_act":  alphaNum,
		"mainfw_type": alphaNum,
		"ro_fwid":     anything,
		"tpm_fwver":   hexNum,
		"tpm_kernver": hexNum,
		"wpsw_boot":   bit,
		"wpsw_cur":    bit,
	}
	checkOutput := func(regExpression string, cmdOutput []byte) bool {
		match, _ := regexp.Match(regExpression, cmdOutput)
		return match
	}

	for subCommand, regExp := range cmdRegexMap {
		cmd := testexec.CommandContext(ctx, crossystemCmd, subCommand)
		output, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			s.Errorf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
		} else if !checkOutput(regExp, output) {
			s.Errorf("%q printed %q, which isn't matched by %q", shutil.EscapeSlice(cmd.Args), output, regExp)
		}
	}
}
