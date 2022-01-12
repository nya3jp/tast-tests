// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mojo

// BTConfigJS is javascript code that initializes the config object and defines
// some additional functions
const BTConfigJS = `function() {
    return {
        bluetoothConfig: chromeos.bluetoothConfig.mojom.CrosBluetoothConfig.getRemote(),

        // for interface SystemPropertiesObserver
        // OnPropertiesUpdated(BluetoothSystemProperties properties);
        onPropertiesUpdated: function(properties) {
            this.systemProperties_ = {}
            this.systemProperties_.systemState = properties.systemState;
            this.systemProperties_.modificationState = properties.modificationState;
        },


        // Initialization
        initSysPropObs: function() {
            this.SysPropObsReceiver = new chromeos.bluetoothConfig.mojom.SystemPropertiesObserverReceiver(this);
            this.bluetoothConfig.observeSystemProperties(this.SysPropObsReceiver.$.bindNewPipeAndPassRemote());
        },

    }
}`
