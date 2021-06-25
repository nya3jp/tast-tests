// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package global

import (
	"testing"
)

// TestCheckDuplicateVars makes sure that there are no duplicate variable in globalVars
func TestCheckDuplicateVars(t *testing.T) {
	varNames := make(map[string]struct{})
	for _, v := range Vars {
		if _, found := varNames[v.Name()]; found {
			t.Errorf("variable %v is defined more than once", v.Name())
		}
		varNames[v.Name()] = struct{}{}
	}
}
