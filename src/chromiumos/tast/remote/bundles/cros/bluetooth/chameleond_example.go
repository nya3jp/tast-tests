// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	pbchameleond "go.chromium.org/chromiumos/config/go/platform/chameleon/chameleond/rpc"

	"chromiumos/tast/remote/bluetooth"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChameleondExample,
		Desc: "Example test that shows how to make Chameleond calls with the CFT ChameleondManagerService",
		Contacts: []string{
			"jaredbennett@google.com",
		},
		Attr:         []string{},
		ServiceDeps:  []string{},
		SoftwareDeps: []string{},
		Fixture:      "bluetoothChameleond",
		Timeout:      1 * time.Minute,
	})
}

func ChameleondExample(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*bluetooth.TestFixture)
	if _, err := tf.CMS.ChameleondService.ResetBluetoothRef(ctx, &pbchameleond.ResetBluetoothRefRequest{}); err != nil {
		s.Fatal("Failed ResetBluetoothRef example call: ", err)
	}
}
