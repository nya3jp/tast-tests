// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cros implements implements unit test for the test metadata in cros bundle.
package main

import (
	gotesting "testing"

	"chromiumos/tast/restrictions"
	"chromiumos/tast/testing/testcheck"
)

const (
	mainlineAttributeName = "group:mainline"
)

func TestEntityLabelInheritance(t *gotesting.T) {
	nodes := testcheck.Entities()
	for _, v := range nodes {
		if v.Parent == "" {
			continue
		}
		p, ok := nodes[v.Parent]
		if !ok {
			t.Errorf("fixture %s, referenced by %s %s, not found", v.Parent, v.Type, v.Name)
			continue
		}
		if !v.HasPrivateAttr(restrictions.FragileUIMatcherLabel) &&
			p.HasPrivateAttr(restrictions.FragileUIMatcherLabel) {
			t.Errorf("%s %s doesn't have \"%s\" privateAttr, but the parent %s %s does.",
				v.Type, v.Name, restrictions.FragileUIMatcherLabel, p.Type, p.Name)
		}
	}
}

func TestMainlineTestsHaveNoFragileUIMatcher(t *gotesting.T) {
	nodes := testcheck.Entities()
	for _, v := range nodes {
		if v.HasPrivateAttr(restrictions.FragileUIMatcherLabel) &&
			v.HasAttr(mainlineAttributeName) {
			t.Errorf("test %s, has \"%s\" privateAttr, but is a mainline test",
				v.Name, restrictions.FragileUIMatcherLabel)
		}
	}
}
