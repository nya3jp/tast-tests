// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

type animal struct {
	numLegs int
	crying  string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     Param,
		Desc:     "Parameterized test example",
		Contacts: []string{"nya@chromium.org", "tast-owners@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "dog",
			Val: animal{
				numLegs: 4,
				crying:  "bow-wow",
			},
		}, {
			Name: "duck",
			Val: animal{
				numLegs: 2,
				crying:  "quack",
			},
		}},
	})
}

func Param(ctx context.Context, s *testing.State) {
	s.Log("Value: ", s.Param().(animal))
}
