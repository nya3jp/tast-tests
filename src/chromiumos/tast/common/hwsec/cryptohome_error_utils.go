// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	uda "chromiumos/system_api/user_data_auth_proto"
)

// ContainsPossibleAction examines the given CryptohomeErrorInfo and see if it contains the specified possible action.
func ContainsPossibleAction(info *uda.CryptohomeErrorInfo, action uda.PossibleAction) bool {
	if info.PrimaryAction != uda.PrimaryAction_PRIMARY_NONE {
		// If the PrimaryAction is not PrimaryNone, then PossibleAction is not used, so it's always false.
		return false
	}

	for _, a := range info.PossibleActions {
		if a == action {
			return true
		}
	}
	return false
}
