// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package set

import (
	"reflect"
	"testing"
)

func TestStringSliceDiff(t *testing.T) {
	a := []string{"a.0.dmp", "a.1.dmp", "b.0.dmp", "c.0.dmp"}
	b := []string{"a.0.dmp", "a.1.dmp", "a.2.dmp"}

	actual := DiffStringSlice(a, b)
	expected := []string{"b.0.dmp", "c.0.dmp"}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Unexpected values: got %v, want %v", actual, expected)
	}
}
