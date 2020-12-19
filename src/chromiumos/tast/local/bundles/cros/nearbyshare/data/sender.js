// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/**
 * @fileoverview JavaScript to run on chrome://nearby to control the sending
 * flow. This is largely based on chrome://nearby's discovery_manager.js and
 * nearby_discovery_page.js with minor adjustments made to ease testing.
 * See https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/resources/nearby_share
 */
function () {
  return {
    // Current transfer status once the share starts
    // (nearbyShare.mojom.TransferStatus).
    currentTransferStatus: null,

    // Result of selectShareTarget (nearbyShare.mojom.SelectShareTargetResult).
    selectShareTargetRes: null,

    // Secure confirmation token shown to both parties. Becomes a valid value
    // once transferUpdateListener_ starts getting updates.
    confirmationToken: null,

    // ConfirmationManager interface for the share target.
    // Becomes a valid object once we have successfully selected a share target.
    confirmationManager_: null,

    // TransferUpdateListener interface for the share target.
    // Becomes a valid object once we have successfully selected a share target.
    transferUpdateListener_: null,

    // Find the first share target with the given name within the discovery
    // page's shareTargets_ array.
    getShareTarget(name) {
      shareTargets = document.querySelector("nearby-share-app").shadowRoot
        .querySelector("nearby-discovery-page").shareTargets_;
      target = null;
      shareTargets.forEach(t => {
        if (t.name == name) {
          target = t;
        }
      });
      return target;
    },

    selectShareTarget(name) {
      shareTarget = this.getShareTarget(name);
      if (!shareTarget) {
        return;
      }
      import('./discovery_manager.js')
        .then(m => m.getDiscoveryManager().selectShareTarget(shareTarget.id))
        .then(response => {
          const {
            result,
            transferUpdateListener,
            confirmationManager
          } = response;
          this.selectShareTargetRes = result;
          if (result !== nearbyShare.mojom.SelectShareTargetResult.kOk) {
            return;
          }

          this.confirmationManager_ = confirmationManager;
          this.transferUpdateListener_ =
            new nearbyShare.mojom.TransferUpdateListenerReceiver(this);
          this.transferUpdateListener_.$.bindHandle(
            transferUpdateListener.handle);
        });
    },

    onTransferUpdate(status, token) {
      if (token) {
        this.confirmationToken = token;
      }
      this.currentTransferStatus = status;
    },

    accept() {
      this.confirmationManager_.accept();
    },

    reject() {
      this.confirmationManager_.reject();
    },

    cancel() {
      this.confirmationManager_.cancel();
    },
  }
}