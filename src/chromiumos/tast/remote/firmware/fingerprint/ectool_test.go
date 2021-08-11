// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import "testing"

func TestRollbackStateEctoolUnmarshaler(t *testing.T) {
	// It should not matter that there are tabs to the left of the examples out.
	var out = []byte(`
	Rollback block id:    19
	Rollback min version: 0
	RW rollback version:  255
	`)
	var rExpect = RollbackState{BlockID: 19, MinVersion: 0, RWVersion: 255}

	var r RollbackState
	if err := r.UnmarshalerEctool(out); err != nil {
		t.Fatal("Failed to unmarshal ")
	}

	if r != rExpect {
		t.Fatalf("Unmarshaled rollback block %+v doesn't match expected block %+v.", r, rExpect)
	}
}
