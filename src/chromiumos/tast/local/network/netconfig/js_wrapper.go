// Copyright 2022 The ChromiumOS Authors
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
  const networkConfigMojoModule = await import('chrome://resources/mojo/chromeos/services/network_config/public/mojom/cros_network_config.mojom-webui.js');

  return {
    crosNetworkConfig_: null,

    getCrosNetworkConfig() {
      if (!this.crosNetworkConfig_) {
        this.crosNetworkConfig_ = networkConfigMojoModule.CrosNetworkConfig.getRemote();
      }
      return this.crosNetworkConfig_;
    },

    async getManagedProperties(guid) {
      response = await this.getCrosNetworkConfig().getManagedProperties(guid);

      // Delete mojo uint64 typed properties because BigInt cannot be
      // serialized.
      delete response.result.trafficCounterProperties.lastResetTime;

      return response.result;
    },

    async configureNetwork(properties, shared) {
      return await this.getCrosNetworkConfig().configureNetwork(properties, shared);
    },

    async forgetNetwork(guid) {
      return await this.getCrosNetworkConfig().forgetNetwork(guid);
    },

    async setNetworkTypeEnabledState(networkType, enable) {
      return await this.getCrosNetworkConfig().setNetworkTypeEnabledState(networkType, enable);
    },

   async getDeviceStateList() {
     return await this.getCrosNetworkConfig().getDeviceStateList();
   },

   async getNetworkStateList(filter) {
     // Filtering is not working unless the first letter is in lower case
     lowercased_filter = {}
     Object.keys(filter).map(function(key,_) {
       lowercased_filter[key[0].toLowerCase()+key.slice(1)] = filter[key]});
     return await this.getCrosNetworkConfig().getNetworkStateList(lowercased_filter);
   },

  }
}
`
