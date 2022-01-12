// Copyright 2022 The Chromium OS Authors. All rights reserved.
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

//SetBluetoothEnabledState sets Bluetooth state to on/off
func SetBluetoothEnabledState(ctx context.Context, s *testing.State, btmojo chrome.JSObject, enabled bool) error {
	js := fmt.Sprintf(`function() {this.bluetoothConfig.setBluetoothEnabledState(%t)}`, enabled)
	if err := btmojo.Call(ctx, nil, js); err != nil {
		return errors.Wrap(err, "setBluetoothEnabledState call failed")
	}
	return nil
}
