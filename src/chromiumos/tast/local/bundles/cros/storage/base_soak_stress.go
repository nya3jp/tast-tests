// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/storage/stress"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BaseSoakStress,
		Desc:         "Performs soak test of a main SSD storage device",
		Contacts:     []string{"chromeos-engprod-platform-syd@google.com"},
		Attr:         []string{"group:storage-qual"},
		Timeout:      60 * time.Minute,
		Data:         stress.Configs,
		SoftwareDeps: []string{"storage_wearout_detect"},
	})
}

// BaseSoakStress runs disk IO performance tests by running the tool "fio".
func BaseSoakStress(ctx context.Context, s *testing.State) {
	// Below sequence of tests corresponds to a single iteration of the soak test.
	stress.RunFioStress(ctx, s, "64k_stress", false)
	stress.Suspend(ctx)
	stress.RunFioStress(ctx, s, "surfing", false)
	stress.Suspend(ctx)
	stress.RunFioStress(ctx, s, "8k_async_randwrite", false)
	stress.RunFioStress(ctx, s, "8k_async_randwrite", true)
}
