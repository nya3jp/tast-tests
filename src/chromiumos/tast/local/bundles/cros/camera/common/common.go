// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"strconv"

	"chromiumos/tast/testing"
)

// IntVar returns the default value if no vars are passed else parses the var to int and returns.
func IntVar(s *testing.State, name string, defaultValue int) int {
	str, ok := s.Var(name)
	if !ok {
		return defaultValue
	}

	val, err := strconv.Atoi(str)
	if err != nil {
		s.Fatalf("Failed to parse integer variable %v: %v", name, err)
	}

	return val
}
