// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	gotesting "testing"

	"chromiumos/tast/common/testing"
)

func MyFunc(*testing.State) {}

func TestListDataFiles(t *gotesting.T) {
	testing.ClearForTesting()
	defer testing.ClearForTesting()

	testing.AddTest(&testing.Test{
		Name: "foo.Test1",
		Func: MyFunc,
		Data: []string{"1"},
	})
	testing.AddTest(&testing.Test{
		Name: "foo.Test2",
		Func: MyFunc,
		Data: []string{"1", "2"},
	})

	tests := testing.GlobalRegistry().AllTests()
	b := bytes.Buffer{}
	if err := listDataFiles(&b, tests); err != nil {
		t.Fatalf("listDataFiles(b, %v) failed: %v", tests, err)
	}

	exp := []string{
		filepath.Join(tests[0].DataDir(), "1"),
		filepath.Join(tests[1].DataDir(), "2"),
	}
	act := make([]string, 0)
	if err := json.Unmarshal(b.Bytes(), &act); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exp, act) {
		t.Errorf("listDataFiles(b, %v) wrote %v; want %v", tests, act, exp)
	}
}
