// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/**
 * @fileoverview A wrapper file around the diagnostics API.
 */
(function () {
  return {
    /**
     * SystemDataProvider mojo remote.
     * @private {?ash.diagnostics.mojom.SystemDataProviderRemote}
     */
    systemDataProvider_: null,

    async getSystemDataProvider() {
      if (!this.systemDataProvider_) {
        module = await import("./system_data_provider.mojom-webui.js");
        this.systemDataProvider_ = module.SystemDataProvider.getRemote();
      }
      return this.systemDataProvider_;
    },

    async fetchSystemInfo() {
      const provider = await this.getSystemDataProvider();
      const result = provider.getSystemInfo();
      // Log for debug purpose.
      console.log("result.systemInfo from tast: ", result.systemInfo);
      return result.systemInfo;
    }
  }
})
