// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package listset provides operations on lists to crostini test
package listset

import (
	"reflect"
	"sort"

	"chromiumos/tast/errors"
)

// CheckListsMatch checks whether two lists equal.
func CheckListsMatch(actual []string, expected ...string) error {
	// Sort and compare the two lists.
	sort.Strings(expected)
	sort.Strings(actual)
	if !reflect.DeepEqual(expected, actual) {
		return errors.Errorf("failed to verify lists, got %s, want %s", actual, expected)
	}
	return nil
}
