// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/bundles/cros/platform/storageinfo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StorageWearoutDetect,
		Desc: "Fails if storage device information indicates impending failure",
		Contacts: []string{
			"puthik@chromium.org",       // Autotest author
			"brooke.mylander@intel.com", // Migrated Autotest to Tast
		},
		Attr: []string{"informational"},
	})
}

func StorageWearoutDetect(ctx context.Context, s *testing.State) {
	info, err := storageinfo.Get(ctx)
	if err != nil {
		s.Fatal("Failed to get storage info: ", err)
	}

	if info.Status == storageinfo.Failing {
		s.Error("Storage device is failing, consider removing from DUT farm")
	}
}
