// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/remote/example"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RemotePre,
		Desc:     "Demonstrates remote precondition works",
		Contacts: []string{"oka@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
		Pre:      example.HelloPre(),
	})
}

func RemotePre(ctx context.Context, s *testing.State) {
	path := s.PreValue().(string)
	b, err := s.DUT().Command("cat", path).Output(ctx)
	if err != nil {
		s.Fatal("reading prepared file: ", err)
	}
	if got, want := string(b), "Hello\n"; got != want {
		s.Errorf("Got content in %s %q; want %q", path, got, want)
	}
}
