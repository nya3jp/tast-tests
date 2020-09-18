// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package genparams

import (
	"testing"

	"github.com/kylelemons/godebug/diff"
)

func TestTemplate(t *testing.T) {
	for _, tc := range []struct {
		text string
		data interface{}
		want string
	}{
		{"abc", nil, "abc"},
		{"{{ . }}", 28, "28"},
		{"{{ . }}", "foo", "foo"},
		{"{{ . | fmt }}", 28, "28"},
		{"{{ . | fmt }}", "foo", `"foo"`},
		{"{{ . | fmt }}", false, "false"},
		{"{{ . | fmt }}", true, "true"},
		{"{{ . | fmt }}", uint32(28), "uint32(28)"},
		{"{{ . | fmt }}", 2.75, "2.75"},
		{"{{ . | fmt }}", float32(2.75), "float32(2.75)"},
		{"{{ . | fmt }}", nil, "nil"},
		{"{{ . | fmt }}", []int(nil), "[]int{}"},
		{"{{ . | fmt }}", []int{}, "[]int{}"},
		{"{{ . | fmt }}", []int{1, 2, 3}, "[]int{1, 2, 3}"},
		{"{{ . | fmt }}", []string{"a", "b", "c"}, `[]string{"a", "b", "c"}`},
		{"{{ . | fmt }}", map[int]int(nil), "map[int]int{}"},
		{"{{ . | fmt }}", map[int]int{}, "map[int]int{}"},
		{"{{ . | fmt }}", map[int]int{1: 11, 2: 22, 3: 33}, "map[int]int{1: 11, 2: 22, 3: 33}"},
		{"{{ . | fmt }}", map[string]string{"a": "x", "b": "y", "c": "z"}, `map[string]string{"a": "x", "b": "y", "c": "z"}`},
		{"{{ . | fmt }}", []map[int]int{{1: 2}, {}, {3: 4}}, "[]map[int]int{{1: 2}, {}, {3: 4}}"},
		{"{{ . | fmt }}", map[int][]int{1: {2, 3}, 4: {}}, "map[int][]int{1: {2, 3}, 4: {}}"},
		{"{{ . | fmt }}", [][][]int{{{28}}}, "[][][]int{{{28}}}"},
	} {
		got := Template(t, tc.text, tc.data)
		if diff := diff.Diff(got, tc.want); diff != "" {
			t.Errorf("Template(%q, %#v) mismatch (-got +want):\n%s", tc.text, tc.data, diff)
		}
	}
}
