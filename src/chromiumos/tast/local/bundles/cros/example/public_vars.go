// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PublicVars,
		Desc:     "Public variables",
		Contacts: []string{"tast-owners@google.com", "oka@chromium.org"},
		Attr:     []string{"group:mainline"},
		// example.PublicVars.foo is defined in tast-tests/vars/example.PublicVars.yaml
		VarDeps: []string{"example.PublicVars.foo"},
	})
}

func PublicVars(ctx context.Context, s *testing.State) {
	if x := s.RequiredVar("example.PublicVars.foo"); x != "bar" {
		s.Errorf(`Got %q, want "bar"`, x)
	}
}
