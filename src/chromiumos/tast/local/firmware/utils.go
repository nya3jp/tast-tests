// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"fmt"
	"regexp"

	"golang.org/x/net/context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// BootModeNormal, BootModeDev, and BootModeRecovery refer to the three boot modes of a powered-on DUT.
const (
	BootModeNormal   = iota
	BootModeDev      = iota
	BootModeRecovery = iota
)

// CheckCrossystemValues calls `crossystem` to check whether the specified key-value pairs are present.
func CheckCrossystemValues(ctx context.Context, values map[string]string) (bool, error) {
	cmd := testexec.CommandContext(ctx, "crossystem")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to run crossystem")
	}
	for k, v := range values {
		s := fmt.Sprintf("(\n|^)%s += %s +#", k, v)
		r := regexp.MustCompile(s)
		if !r.Match(out) {
			return false, nil
		}
	}
	return true, nil
}

// CheckBootMode determines whether the DUT is in the specified boot mode based on crossystem values.
func CheckBootMode(ctx context.Context, mode int) (bool, error) {
	var crossystemValues map[string]string
	switch mode {
	case BootModeNormal:
		crossystemValues = map[string]string{"devsw_boot": "0", "mainfw_type": "normal"}
	case BootModeDev:
		crossystemValues = map[string]string{"devsw_boot": "1", "mainfw_type": "developer"}
	case BootModeRecovery:
		crossystemValues = map[string]string{"mainfw_type": "recovery"}
	default:
		return false, errors.Errorf("unrecognized boot mode %d", mode)
	}
	verified, err := CheckCrossystemValues(ctx, crossystemValues)
	if err != nil {
		return false, errors.Wrap(err, "during CheckBootMode")
	}
	return verified, nil
}
