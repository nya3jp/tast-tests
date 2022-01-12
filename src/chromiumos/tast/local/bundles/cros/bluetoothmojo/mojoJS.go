// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetoothmojo

//BTConfigJS is javascript code that returns mojo object and some helper functions
const BTConfigJS = `function { return {
                 bluetoothConfig : chromeos.bluetoothConfig.mojom.CrosBluetoothConfig.getRemote(),
             }}`
