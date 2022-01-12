// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetoothmojo

// JS code that return mojo JS object and some helper functions
const BTConfigJS = `function { return {
                 bluetoothConfig : chromeos.bluetoothConfig.mojom.CrosBluetoothConfig.getRemote(),
             }}`
