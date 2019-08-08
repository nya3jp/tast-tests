// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Vars,
		Desc:     "Test vars",
		Contacts: []string{"tast-owners@google.com"},
		Attr:     []string{"informational"},
		// example.Vars.foo is defined in tast-tests/vars/example.Vars.yaml
		// example.Vars.secret is defined in tast-tests-private/vars/example.Vars.yaml
		// FIXME: add link to document.
		Vars: []string{"example.Vars.foo", "example.Vars.secret"},
	})
}

func Vars(ctx context.Context, s *testing.State) {
	if x := s.Var("example.Vars.foo"); x != "42" {
		s.Errorf(`Foo got %q, want "42"`, x)
	}
	if x := s.Var("example.Vars.secret"); x != "passw0rd" {
		s.Errorf(`Secret got %q, want "passw0rd"`, x)
	}
}
