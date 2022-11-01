// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package networkhealth

// Stores network_health remote in between calls. This can be exported to a
// file as soon as file dependencies may be passed to grpc services.
const networkHealthServiceJs = `
/**
 * @fileoverview A wrapper file for the network health API.
 */
async function() {
	return {
		/**
		* Network Health mojo remote.
		* @private {
		*	?chromeos.networkHealth.mojom.NetworkHealthService}
		*/
		networkHealth_: null,

		getNetworkHealth() {
			if (!this.networkHealth_) {
				// Check ash namespace and fallback to chromeos if unavailable, for renaming.
				// TODO(crbug.com/1255018): Remove the fallback once the renaming is completed.
				let has_ash_mojom = typeof ash !== "undefined" &&
				    typeof ash.networkHealth !== "undefined";
				this.networkHealth_ = has_ash_mojom ?
				ash.networkHealth.mojom.NetworkHealthService.getRemote() :
				chromeos.networkHealth.mojom.NetworkHealthService.getRemote();
			}
			return this.networkHealth_;
		},

		async getNetworkList() {
			return await this.getCrosNetworkConfig().getNetworkList();
		},
	}
}
`
