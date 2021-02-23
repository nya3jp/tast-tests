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
		Func:     Run,
		Desc:     "Subtest example, always fails",
		Contacts: []string{"vsavu@google.com", "tast-owners@google.com"},
	})
}

func Run(ctx context.Context, s *testing.State) {
	for retries := 0; retries < 5; retries++ {
		s.Logf("Subtest: %s, retry: %d of %d", "stest", retries+1, 5)
		passed := s.Run(ctx, "stest", func(ctx context.Context, s *testing.State) {
			if retries < 3 {
				s.Fatal("Not yet")
			}
		})
		if passed {
			break
		}
	}
}
