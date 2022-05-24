// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoteSkip,
		Desc:         "Always skips",
		Contacts:     []string{"tast-owners@google.com"},
		HardwareDeps: hwdep.D(hwdep.Model()),
	})
}

func RemoteSkip(ctx context.Context, s *testing.State) {
}
