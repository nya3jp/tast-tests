// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dep

import (
	gotesting "testing"
)

func TestAsusListIsSorted(t *gotesting.T) {
	for i := 1; i < len(asusModelList); i++ {
		if prev, cur := asusModelList[i-1], asusModelList[i]; prev >= cur {
			t.Errorf("Asus models are not in alphabetical order or contain duplicates: %q is followed by %q", prev, cur)
		}
	}
}
