// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/errors"
)

// CheckForPossibleAction examines the given CryptohomeErrorInfo and see if it contains the specified possible action.
func CheckForPossibleAction(info *uda.CryptohomeErrorInfo, action uda.PossibleAction) error {
	if info.PrimaryAction != uda.PrimaryAction_PRIMARY_NONE {
		// If the PrimaryAction is not PrimaryNone, then PossibleAction is not used, so it's always false.
		return errors.Errorf("expecting PossibleAction %s but PrimaryAction is %s and not PRIMARY_NONE", action.String(), info.PrimaryAction.String())
	}

	for _, a := range info.PossibleActions {
		if a == action {
			return nil
		}
	}
	return errors.Errorf("expecting PossibleAction %s but it's not found", action.String())
}
