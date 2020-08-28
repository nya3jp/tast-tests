// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	_ "chromiumos/tast/local/bundles/cros/meta/fixture" // force import fixtures
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalFixture,
		Desc:     "Runs a tree of local fixtures",
		Contacts: []string{"tast-owners@google.com"},
		Params: []testing.Param{
			{Name: "a", Val: ""},
			{Name: "b", Val: ""},
			{Name: "c", Val: "meta.Parent", Fixture: "meta.Parent"},
			{Name: "d", Val: "meta.Parent", Fixture: "meta.Parent"},
			{Name: "e", Val: "meta.Child1", Fixture: "meta.Child1"},
			{Name: "f", Val: "meta.Child1", Fixture: "meta.Child1"},
			{Name: "g", Val: "meta.Child2", Fixture: "meta.Child2"},
			{Name: "h", Val: "meta.Child2", Fixture: "meta.Child2"},
		},
	})
}

func LocalFixture(ctx context.Context, s *testing.State) {
	p := s.Param().(string)
	var want interface{}
	if p != "" {
		want = p
	}
	if got := s.FixtValue(); got != want {
		s.Errorf("FixtValue = %v; want %v", got, want)
	}
}
