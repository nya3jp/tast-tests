// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package strcmp supports comparing strings.
package strcmp

import "github.com/kylelemons/godebug/pretty"

// SameList compares expected and actual list of strings as sets, that is, checks if they contain the same
// values ignoring the order. It returns a human readable diff between the values, which is an empty
// string if the values are the same.
func SameList(want, got []string) string {
	gotMap := make(map[string]bool)
	for _, g := range got {
		gotMap[g] = true
	}

	wantMap := make(map[string]bool)
	for _, w := range want {
		wantMap[w] = true
	}

	return pretty.Compare(wantMap, gotMap)
}
