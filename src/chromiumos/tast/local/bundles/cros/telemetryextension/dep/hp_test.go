// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dep

import (
	gotesting "testing"
)

func TestHPListIsSorted(t *gotesting.T) {
	for i := 1; i < len(hpModelList); i++ {
		if prev, cur := hpModelList[i-1], hpModelList[i]; prev >= cur {
			t.Errorf("HP models are not in alphabetical order or contain duplicates: %q is followed by %q", prev, cur)
		}
	}
}
