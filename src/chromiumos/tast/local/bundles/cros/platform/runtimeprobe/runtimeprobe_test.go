// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package runtimeprobe

import "testing"

func TestTryTrimQid(t *testing.T) {
	for i, tc := range []struct {
		model, category, input, expected string
	}{
		{ // match name policy
			model:    "model",
			category: "category",
			input:    "model_category_1234_5678",
			expected: "model_category_1234_{Any}",
		},
		{ // match name policy (with seq appended)
			model:    "model",
			category: "category",
			input:    "model_category_1234_5678#12345",
			expected: "model_category_1234_{Any}",
		},
		{ // not match name policy
			model:    "model",
			category: "category",
			input:    "model_category_48d58a8b",
			expected: "model_category_48d58a8b",
		},
		{ // special chars in model/category
			model:    `\.\.\.\.`,
			category: "category",
			input:    `\.\.\.\._category_1234_5678`,
			expected: `\.\.\.\._category_1234_{Any}`,
		},
		{ // special chars (not valid regexp) in model/category
			model:    "[([([(",
			category: "category",
			input:    "[([([(_category_1234_5678",
			expected: "[([([(_category_1234_{Any}",
		},
		{ // category mismatches in comp names
			model:    "model",
			category: "category1",
			input:    "model_category2_1234_5678",
			expected: "model_category2_1234_5678",
		},
		{ // extra chars in the end
			model:    "model",
			category: "category",
			input:    "model_category_1234_5678_9012",
			expected: "model_category_1234_5678_9012",
		},
	} {
		got := tryTrimQid(tc.model, tc.category, tc.input)
		if got != tc.expected {
			t.Errorf("testcase %d failed; input %v; got %v; want %v", i, tc.input, got, tc.expected)
		}
	}
}
