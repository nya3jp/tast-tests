// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements a unit test for cros test bundles.
//
// The test checks the restrictions enforced for the test metadata.
package main

import (
	gotesting "testing"

	_ "chromiumos/tast/local/bundles/cros/allcategories"
	_ "chromiumos/tast/remote/bundles/cros/allcategories"
	"chromiumos/tast/testing/testcheck"
)

const (
	mainlineAttributeName = "group:mainline"
	fragileLabel          = "use_fragile_matcher"
)

func TestFixtTest(t *gotesting.T) {
	nodes := testcheck.EntityDependencies()
	for _, v := range nodes {
		if v.HasLabel(fragileLabel) {
			if v.HasAttr(mainlineAttributeName) {
				t.Errorf("test %s, has \"%s\" label, but is a mainline test",
					v.Name, fragileLabel)
			}
			continue
		}
		if v.Parent == "" {
			continue
		}
		p, ok := nodes[v.Parent]
		if !ok {
			t.Errorf("fixture %s, referenced by %s %s, not found", v.Parent, v.Type, v.Name)
			continue
		}
		if p.HasLabel(fragileLabel) {
			t.Errorf("%s %s doesn't have \"%s\" label, but the parent %s %s does.",
				v.Type, v.Name, fragileLabel, p.Type, p.Name)
		}
	}
}
