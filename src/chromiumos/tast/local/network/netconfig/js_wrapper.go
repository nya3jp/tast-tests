// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netconfig

// Stores cros network config remote in between calls. This can be exported to
// a file as soon as file dependencies may be passed to grpc services.
const crosNetworkConfigJs = `
/**
 * @fileoverview A wrapper file for the cros network config API.
 */
async function() {
  return {
    crosNetworkConfig_: null,

    getCrosNetworkConfig() {
      if (!this.crosNetworkConfig_) {
        this.crosNetworkConfig_ =
          chromeos.networkConfig.mojom.CrosNetworkConfig.getRemote();
      }
      return this.crosNetworkConfig_;
    },

    async getManagedProperties(guid) {
      response = await this.getCrosNetworkConfig().getManagedProperties(guid);

      // Delete mojo uint64 typed properties because BigInt cannot be
      // serialized.
      delete response.result.trafficCounterResetTime;

      return response.result;
    },

    async configureNetwork(properties, shared) {
      return await this.getCrosNetworkConfig().configureNetwork(properties, shared);
    },

    async forgetNetwork(guid) {
      return await this.getCrosNetworkConfig().forgetNetwork(guid);
    },
  }
}
`
