// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mojo

// ESimManagerJS is javascript code that initializes the eSIM manager object and defines
// some additional functions.
const ESimManagerJS = `async function() {
	let esimMojo = await import('chrome://resources/mojo/chromeos/ash/services/cellular_setup/public/mojom/esim_manager.mojom-webui.js')
	return {
		manager: esimMojo.ESimManager.getRemote(),

		async getAvailableEuiccEids() {
			const res = [];
			const response = await this.manager.getAvailableEuiccs();
			for (const euicc of response.euiccs) {
				const properties = (await euicc.getProperties()).properties;
				res.push(properties.eid);
			}

			return res;
		},

		async getEuicc_(eid) {
			const response = await this.manager.getAvailableEuiccs();
			for (const euicc of response.euiccs) {
				const properties = (await euicc.getProperties()).properties;
				if (properties.eid == eid) {
					return euicc;
				}
			}

			return null;
		},

		async getEuiccProperties(eid) {
			const euicc = await this.getEuicc_(eid);
			return (await euicc.getProperties()).properties;
		},

		async getProfileIccids(eid) {
			const res = [];
			const euicc = await this.getEuicc_(eid);
			const response = await euicc.getProfileList();
			for (const profile of response.profiles) {
				const properties = (await profile.getProperties()).properties;
				res.push(properties.iccid);
			}

			return res;
		},

		async requestPendingProfiles(eid) {
			const euicc = await this.getEuicc_(eid);
			return (await euicc.requestPendingProfiles()).result;
		},

		async installProfileFromActivationCode(eid, activationCode, confirmationCode) {
			const euicc = await this.getEuicc_(eid);
			const response = await euicc.installProfileFromActivationCode(activationCode, confirmationCode);

			let iccid = "";
			if (response.profile && typeof response.profile != "undefined") {
				iccid = (await response.profile.getProperties()).properties.iccid;
			}

			return {
				Result: response.result,
				Iccid: iccid
			}
		},

		async getEidQrCode(eid) {
			const euicc = await this.getEuicc_(eid);
			return (await euicc.getEidQRCode()).qrCode;
		},

		async getProfile_(iccid) {
			const euiccs = await this.manager.getAvailableEuiccs();
			for (const euicc of euiccs.euiccs) {
				const response = await euicc.getProfileList();
				for (const profile of response.profiles) {
					const properties = (await profile.getProperties()).properties;
					if (properties.iccid == iccid) {
						return profile;
					}
				}
			}

			return null;
		},

		async getProfileProperties(iccid) {
			const profile = await this.getProfile_(iccid);
			return (await profile.getProperties()).properties;
		},

		async installProfile(iccid, confirmationCode) {
			const profile = await this.getProfile_(iccid);
			return (await profile.installProfile(confirmationCode)).result;
		},

		async uninstallProfile(iccid) {
			const profile = await this.getProfile_(iccid);
			return (await profile.uninstallProfile()).result;
		},

		async setProfileNickname(iccid, name) {
			const profile = await this.getProfile_(iccid);
			return (await profile.setProfileNickname(name)).result
		}
    }
}`
