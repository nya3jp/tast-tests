// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/crosdisks"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CrosDisksArchive,
		Desc:     "Checks that cros-disks can mount various archive types",
		Contacts: []string{"chromeos-files-syd@google.com"},
		Attr:     []string{"group:mainline"},
		Data:     crosdisks.PreparedArchives,
		Timeout:  5 * time.Minute,
	})
}

func CrosDisksArchive(ctx context.Context, s *testing.State) {
	crosdisks.RunArchiveTests(ctx, s)
}
