// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/remote/testlibs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     TestLibs,
		Desc:     "Demonstrates how to connect to a test lib from tast",
		Contacts: []string{"kathrelkeld@chromium.org", "tast-owners@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func TestLibs(ctx context.Context, s *testing.State) {
	// Start a new TestLibService connection (dials via rpc).
	ls, err := testlibs.NewLibsService()
	if err != nil {
		s.Fatal("Could not start TestLibService's client: ", err)
	}
	defer ls.Close(ctx)

	// Start a known library by providing its id.
	l, err := ls.StartLib(ctx, "example_rest_service")
	if err != nil {
		s.Fatal("Could not start lib: ", err)
	}
	defer l.Close(ctx)
}
