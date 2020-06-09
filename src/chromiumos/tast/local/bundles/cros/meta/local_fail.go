// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalFail,
		Desc:     "Always fails",
		Contacts: []string{"tast-owners@google.com"},
	})
}

func LocalFail(ctx context.Context, s *testing.State) {
	s.Error("Failed")
}
