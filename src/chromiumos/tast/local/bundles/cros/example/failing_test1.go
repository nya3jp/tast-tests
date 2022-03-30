// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"chromiumos/tast/testing"
	"context"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     FailingTest1,
		Desc:     "Always fails",
		Contacts: []string{"vsavu@chromium.org", "tast-owners@google.com"},
		Fixture:  "failingfixture",
	})
}

func FailingTest1(ctx context.Context, s *testing.State) {
	s.Log("Running test 1")
}
