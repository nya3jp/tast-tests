// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package util provides utilities to crostini test
package util

import (
	"reflect"
	"sort"

	"chromiumos/tast/errors"
)

// CheckListsMatch checks whether two lists equal.
func CheckListsMatch(realList []string, expectedList ...string) error {
	// Sort and compare the two lists.
	sort.Strings(expectedList)
	sort.Strings(realList)
	if !reflect.DeepEqual(expectedList, realList) {
		return errors.Errorf("failed to verify lists, want %s, got %s", expectedList, realList)
	}
	return nil
}
