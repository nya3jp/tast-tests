// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalFeatures,
		Desc:     "Example to access DUT features from a local test",
		Contacts: []string{"seewaifu@google.com", "tast-owners@google.com"},
	})
}

func LocalFeatures(ctx context.Context, s *testing.State) {
	dutFeatures := s.Features("")
	s.Logf("DUT Features: %+v", dutFeatures)
}
