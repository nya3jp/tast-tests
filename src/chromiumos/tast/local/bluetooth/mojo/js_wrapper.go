// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mojo

// BTConfigJS is javascript code that initializes the config object and defines
// some additional functions.
const BTConfigJS = `async function () {
    let btmojo = await import('chrome://resources/mojo/chromeos/services/bluetooth_config/public/mojom/cros_bluetooth_config.mojom-webui.js');
    return {
        bluetoothConfig : btmojo.CrosBluetoothConfig.getRemote(),

        // for interface SystemPropertiesObserver
        // OnPropertiesUpdated(BluetoothSystemProperties properties);
        onPropertiesUpdated: function(properties) {
            this.systemProperties_ = {}
            this.systemProperties_.systemState = properties.systemState;
            this.systemProperties_.modificationState = properties.modificationState;
        },

        // Initialization
        initSysPropObs: function() {
            this.SysPropObsReceiver = new btmojo.SystemPropertiesObserverReceiver(this);
            this.bluetoothConfig.observeSystemProperties(this.SysPropObsReceiver.$.bindNewPipeAndPassRemote());
        },

    }
}`
