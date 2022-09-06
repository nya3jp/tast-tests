// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/errors"
)

/*
This file implements miscellaneous and unsorted helpers.
*/

// ExpectAuthIntents checks whether two given sets of intents are equal, and
// in case they're not returns an error containing the formatted difference.
func ExpectAuthIntents(intents, expectedIntents []uda.AuthIntent) error {
	less := func(a, b uda.AuthIntent) bool { return a < b }
	diff := cmp.Diff(intents, expectedIntents, cmpopts.SortSlices(less))
	if diff == "" {
		return nil
	}
	return errors.New(diff)
}

// ExpectAuthFactorTypes checks whether two given sets of types are equal, and
// in case they're not returns an error containing the formatted difference.
func ExpectAuthFactorTypes(types, expectedTypes []uda.AuthFactorType) error {
	less := func(a, b uda.AuthFactorType) bool { return a < b }
	diff := cmp.Diff(types, expectedTypes, cmpopts.SortSlices(less))
	if diff == "" {
		return nil
	}
	return errors.New(diff)
}
