// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import "testing"

func TestRollbackStateEctoolUnmarshaler(t *testing.T) {
	// Note that the following test string is not exactly what ectool would
	// emit, since it contains tabs at the beginning of each line and includes
	// a few extra newlines. These tabs and newlines are purely consmetic.
	var out = []byte(`
	Rollback block id:    19
	Rollback min version: 0
	RW rollback version:  255
	`)
	var rExpect = RollbackState{BlockID: 19, MinVersion: 0, RWVersion: 255}

	var r RollbackState
	if err := r.UnmarshalerEctool(out); err != nil {
		t.Fatal("Failed to unmarshal: ", err)
	}

	if r != rExpect {
		t.Fatalf("Unmarshaled rollback block %+v doesn't match expected block %+v.", r, rExpect)
	}
}

func TestRollbackStateEctoolUnmarshalerError(t *testing.T) {
	var out = []byte(`
	Rollback block id:    19
	Rollback min version: 0F
	RW rollback version:  255
	`)

	var r RollbackState
	if err := r.UnmarshalerEctool(out); err == nil {
		t.Fatalf("Failed to detect error in rollback min version. Produced rollback block %+v.", r)
	}
}

func TestParseColonDelimitedOutput(t *testing.T) {
	const ectoolVersionOutput = `

f1: A B C
	 f2	 : D E F
 f3 : G : H : I


`

	vals := parseColonDelimitedOutput(ectoolVersionOutput)
	if keys := len(vals); keys != 3 {
		t.Fatalf("Wrong number of keys. Expected 3, but received %d keys.", keys)
	}
	for k, v := range vals {
		if v == "" {
			t.Fatalf("Key %s had blank value.", k)
		}
	}

	check := func(field, value string) {
		v, ok := vals[field]
		if !ok {
			t.Fatal("Missing field '" + field + "'.")
		}
		if vals[field] != value {
			t.Fatal("Field " + field + " contains invalid entry '" + v + "'.")
		}
		delete(vals, field)
	}

	check("f1", "A B C")
	check("f2", "D E F")
	check("f3", "G : H : I")

	if len(vals) != 0 {
		t.Fatal("Parsed extra values.")
	}
}
