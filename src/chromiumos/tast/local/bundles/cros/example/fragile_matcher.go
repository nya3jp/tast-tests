// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/restrictions"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "exampleFixture",
		Desc:            "An example",
		Contacts:        []string{},
		SetUpTimeout:    30 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 10 * time.Second,
		Labels:          []string{restrictions.FragileUIMatcherLabel},
		// Implementation omitted, because this is just example of test metadata.
	})
	testing.AddTest(&testing.Test{
		Func:     FragileMatcher,
		Desc:     "Fixture uses fragile UI matcher",
		Contacts: []string{"nya@chromium.org", "tast-owners@google.com"},
		Attr:     []string{"group:mainline"},
		Fixture:  "exampleFixture",
		Params: []testing.Param{{
			// Unit test fails, because the depended fixture uses fragile UI matcher
		}, {
			// Unit test fails, because the test is mainline
			Name:        "allow",
			ExtraLabels: []string{restrictions.FragileUIMatcherLabel},
		}},
	})
}

func FragileMatcher(ctx context.Context, s *testing.State) {
	// No errors means the test passed.
}
