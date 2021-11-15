// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cros implements implements unit test for the test metadata in cros bundle.
package cros

import (
	gotesting "testing"

	_ "chromiumos/tast/local/bundles/cros/allcategories"
	_ "chromiumos/tast/remote/bundles/cros/allcategories"
	"chromiumos/tast/restrictions"
	"chromiumos/tast/testing/testcheck"
)

const (
	mainlineAttributeName = "group:mainline"
)

func TestEntityLabels(t *gotesting.T) {
	nodes := testcheck.EntityDependencies()
	for _, v := range nodes {
		if v.HasLabel(restrictions.FragileUIMatcherLabel) {
			if v.HasAttr(mainlineAttributeName) {
				t.Errorf("test %s, has \"%s\" label, but is a mainline test",
					v.Name, restrictions.FragileUIMatcherLabel)
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
		if p.HasLabel(restrictions.FragileUIMatcherLabel) {
			t.Errorf("%s %s doesn't have \"%s\" label, but the parent %s %s does.",
				v.Type, v.Name, restrictions.FragileUIMatcherLabel, p.Type, p.Name)
		}
	}
}
