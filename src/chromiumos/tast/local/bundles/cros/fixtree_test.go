// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	gotesting "testing"

	// "chromiumos/tast/testing"
	"chromiumos/tast/testing/testcheck"
	// "strings"
)

func TestFixtTest(t *gotesting.T) {
	// This catches errors (e.g. naming issues) encountered during test registration,
	// which is performed by init() functions in test packages that are pulled in by imports in main.go.
	// for _, err := range testing.RegistrationErrors() {
	// 	t.Error("Test registration failed:", err)
	// }
	// f := func(t *testing.TestInstance) bool {
	// 	return strings.HasPrefix(t.Name, "example.")
	// 	// return false
	// }
	testcheck.FixtureDeps(t)
}
