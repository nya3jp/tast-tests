// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"fmt"

	"golang.org/x/net/context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// BootMode is enum of the possible DUT states (besides OFF).
type BootMode int

// DUTs have three possible boot modes: Normal, Dev, and Recovery.
const (
	BootModeNormal   BootMode = iota
	BootModeDev      BootMode = iota
	BootModeRecovery BootMode = iota
)

// CheckCrossystemValues calls crossystem to check whether the specified key-value pairs are present.
// We use the following crossystem syntax, which returns an error code of 0
// if (and only if) all key-value pairs match:
//     crossystem param1?value1 [param2?value2 [...]]
func CheckCrossystemValues(ctx context.Context, values map[string]string) bool {
	cmdArgs := make([]string, len(values))
	i := 0
	for k, v := range values {
		cmdArgs[i] = fmt.Sprintf("%s?%s", k, v)
		i++
	}
	cmd := testexec.CommandContext(ctx, "crossystem", cmdArgs...)
	_, err := cmd.Output(testexec.DumpLogOnError)
	return err == nil
}

// CheckBootMode determines whether the DUT is in the specified boot mode based on crossystem values.
func CheckBootMode(ctx context.Context, mode BootMode) (bool, error) {
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
	return CheckCrossystemValues(ctx, crossystemValues), nil
}
