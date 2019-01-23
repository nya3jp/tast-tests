// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/bundles/cros/platform/verity"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DMVerity,
		Desc: "Verify dm-verity should IO errors on bad data",
		Attr: []string{"informational"},
	})
}

func DMVerity(ctx context.Context, s *testing.State) {
	verity.RunTests(ctx, s)
}
