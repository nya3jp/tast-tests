// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	crossystemCmd = "crossystem"
	alphaNum      = "[\\d\\w]+"
	num           = "[\\d]+"
	hexaNum       = "0x[\\da-fA-F]+"
	bit           = "[01]"
	anything      = "!(\\(error\\))|^" // anything but "(error)" or ""
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckCrossystem,
		Desc: "Checks that the crossystem command basic functionality is present",
		Contacts: []string{
			"semenzato@chromium.org",     // Autotest author
			"kasaiah.bogineni@intel.com", //Port author
			"tast-users@chromium.org"},
		Attr: []string{"disabled", "informational"},
	})
}

func isOutpuExpected(regExp string, cmdOutput []byte) bool {
	isMatched := true
	if strings.HasPrefix(regExp, "!") {
		regExp = strings.Split(regExp, "!")[1]
		isMatched = false
	}
	match, _ := regexp.Match("^"+regExp+"$", cmdOutput)
	return isMatched == match
}

// CheckCrossystem checks that the "crossystem" command basic functionality is present.
// This includes commands that rely on the presence and correct
// initialization of the chromeos driver (drivers/platform/chromeos.c)
// in the kernel
func CheckCrossystem(ctx context.Context, s *testing.State) {
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
		"tpm_fwver":   hexaNum,
		"tpm_kernver": hexaNum,
		"wpsw_boot":   bit,
		"wpsw_cur":    bit,
	}

	for subCommand, regExp := range cmdRegexMap {
		output, err := testexec.CommandContext(ctx, crossystemCmd, subCommand).Output(testexec.DumpLogOnError)
		if err != nil {
			s.Errorf("%s %s failed: err is %v ", crossystemCmd, subCommand, err)
		} else if !isOutpuExpected(regExp, output) {
			s.Errorf("%s %s output is not as expected. Output is %s", crossystemCmd, subCommand, output)
		}
	}
}
