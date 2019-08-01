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
		Func:     Param,
		Desc:     "Parametric test example",
		Contacts: []string{"tast-owners@google.com"},
		Attr:     []string{"informational"},
		Params: []testing.Param{{
			Name: "param1",
			Val:  10,
		}, {
			Name: "param2",
			Val:  20,
		}},
	})
}

func Param(ctx context.Context, s *testing.State) {
	s.Logf("Value: %d", s.Param().(int))
}
