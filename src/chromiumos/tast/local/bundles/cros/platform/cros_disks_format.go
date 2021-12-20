// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/bundles/cros/platform/crosdisks"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosDisksFormat,
		Desc: "Verifies CrosDisks formats removable media",
		Contacts: []string{
			"chromeos-files-syd@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func CrosDisksFormat(ctx context.Context, s *testing.State) {
	crosdisks.RunFormatTests(ctx, s)
}
