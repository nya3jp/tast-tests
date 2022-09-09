// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dep

import (
	gotesting "testing"
)

func TestStableAndNonStableHaveNoIntersection(t *gotesting.T) {
	for _, s := range stableModelList {
		for _, n := range nonStableModelList {
			if s == n {
				t.Errorf("Stable and non-stable have common item: %q", s)
			}
		}
	}
}

func TestStableModelsAllowedOEMs(t *gotesting.T) {
	var allowedModels []string
	allowedModels = append(allowedModels, asusModelList...)
	allowedModels = append(allowedModels, hpModelList...)

	for _, s := range stableModelList {
		allowed := false
		for _, a := range allowedModels {
			if s == a {
				allowed = true
			}
		}
		if !allowed {
			t.Errorf("Stable model (%q) is not allowed OEM model", s)
		}
	}
}

func TestNonStableModelsAllowedOEMs(t *gotesting.T) {
	var allowedModels []string
	allowedModels = append(allowedModels, asusModelList...)
	allowedModels = append(allowedModels, hpModelList...)

	for _, n := range nonStableModelList {
		allowed := false
		for _, a := range allowedModels {
			if n == a {
				allowed = true
			}
		}
		if !allowed {
			t.Errorf("Non-stable model (%q) is not allowed OEM model", n)
		}
	}
}
