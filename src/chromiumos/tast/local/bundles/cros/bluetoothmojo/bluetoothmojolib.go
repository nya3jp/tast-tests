// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetoothmojo

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"context"
	"fmt"
)

// Wrapper functions around Bluetooth mojo JS calls

// Set Bluetooth state to On/Off
func SetBluetoothEnabledState(btmojo chrome.JSObject, ctx context.Context, s *testing.State, enabled bool) error {
	js := fmt.Sprintf(`function() {this.bluetoothConfig.setBluetoothEnabledState(%t)}`, enabled)
	if err := btmojo.Call(ctx, nil, js); err != nil {
		return errors.Wrap(err, "setBluetoothEnabledState call failed")
	}
	return nil
}
