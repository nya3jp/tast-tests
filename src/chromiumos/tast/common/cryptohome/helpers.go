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

// ExpectContainsAuthIntent checks whether the intents set contains the given value.
func ExpectContainsAuthIntent(intents []uda.AuthIntent, expectedIntent uda.AuthIntent) error {
	for _, intent := range intents {
		if intent == expectedIntent {
			return nil
		}
	}
	return errors.Errorf("expected to contain %v, got %v", expectedIntent, intents)
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

// ExpectAuthFactorsWithTypeAndLabel checks whether AuthFactorWithStatus proto
// contains expected AuthFactors, looking only at the types and labels of the
// factors. If they are not equal then this returns an error containing the
// formatted difference.
func ExpectAuthFactorsWithTypeAndLabel(factors, expectedFactors []*uda.AuthFactorWithStatus) error {
	eq := func(a, b *uda.AuthFactorWithStatus) bool {
		return a.AuthFactor.Type == b.AuthFactor.Type && a.AuthFactor.Label == b.AuthFactor.Label
	}
	less := func(a, b *uda.AuthFactorWithStatus) bool {
		return a.AuthFactor.Type < b.AuthFactor.Type || (a.AuthFactor.Type == b.AuthFactor.Type && a.AuthFactor.Label < b.AuthFactor.Label)
	}
	diff := cmp.Diff(factors, expectedFactors, cmp.Comparer(eq), cmpopts.SortSlices(less))
	if diff == "" {
		return nil
	}
	return errors.New(diff)
}
