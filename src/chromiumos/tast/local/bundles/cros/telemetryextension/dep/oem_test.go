// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dep

import (
	gotesting "testing"
)

func TestAsusHPHaveNoIntersection(t *gotesting.T) {
	for _, a := range asusModelList {
		for _, h := range hpModelList {
			if a == h {
				t.Errorf("Asus and HP have common item: %q", a)
			}
		}
	}
}
