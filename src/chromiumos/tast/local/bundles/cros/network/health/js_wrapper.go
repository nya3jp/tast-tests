// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

// Stores network_health remote in between calls. This can be exported to a
// file as soon as file dependencies may be passed to grpc services.
const networkHealthJs = `
/**
 * @fileoverview A wrapper file for the network health mojo API. This is a
 * sparse API wrapper. Only the methods used in tests have been added.
 */
async function() {
	const networkHealthMojoModule = await import('chrome://resources/mojo/chromeos/services/network_health/public/mojom/network_health.mojom-webui.js');

	return {
		networkHealth_: null,

		getNetworkHealth() {
		     if (!this.networkHealth_) {
			     this.networkHealth_ = networkHealthMojoModule.NetworkHealthService.getRemote();
		     }
		     return this.networkHealth_;
		},

		async getNetworkList() {
			return await this.getNetworkHealth().getNetworkList();
		},
	}
}
`
