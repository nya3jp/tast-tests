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
    // Names of discovered share targets we can send to. Populated by the
    // discoverManager_'s ShareTargetListener when share targets are lost/found.
    shareTargetNameMap: new Map(),

    // Current transfer status once the share starts
    // (nearbyShare.mojom.TransferStatus).
    currentTransferStatus: null,

    // Result of startDiscovery (nearbyShare.mojom.StartDiscoveryResult).
    startDiscoveryRes: null,

    // Result of selectShareTarget (nearbyShare.mojom.SelectShareTargetResult).
    selectShareTargetRes: null,

    // Secure confirmation token shown to both parties. Becomes a valid value
    // once transferUpdateListener_ starts getting updates.
    confirmationToken: null,

    // DiscoveryManager interface for testing, whose purpose is to expose
    // discovered share targets to automated tests. The underlying
    // chrome://nearby/ page's DiscoveryManager interface will be used to
    // actually perform the share, but it does not publicly expose its share
    // targets, so this additional interface is required.
    discoverManager_: null,

    // ConfirmationManager interface for the share target.
    // Becomes a valid object once we have successfully selected a share target.
    confirmationManager_: null,

    // TransferUpdateListener interface for the share target.
    // Becomes a valid object once we have successfully selected a share target.
    transferUpdateListener_: null,

    mojoEventTarget_: null,

    listenerIDs_: null,

    getDiscoveryManager_() {
      if (this.discoveryManager_) {
        return this.discoveryManager_
      }
      this.discoveryManager_ = nearbyShare.mojom.DiscoveryManager.getRemote();
      this.discoveryManager_.onConnectionError.addListener(
        () => this.discoveryManager_ = null);
      return this.discoveryManager_;
    },

    startDiscovery() {
      this.mojoEventTarget_ =
        new nearbyShare.mojom.ShareTargetListenerCallbackRouter();

      this.listenerIds_ = [
        this.mojoEventTarget_.onShareTargetDiscovered.addListener(
          this.onShareTargetDiscovered_.bind(this)),
        this.mojoEventTarget_.onShareTargetLost.addListener(
          this.onShareTargetLost_.bind(this)),
      ];

      this.getDiscoveryManager_()
        .startDiscovery(this.mojoEventTarget_.$.bindNewPipeAndPassRemote())
        .then(response => this.startDiscoveryRes = response.result);
    },

    stopDiscovery() {
      if (!this.mojoEventTarget_) {
        return;
      }

      this.shareTargetNameMap.clear();
      this.listenerIds_.forEach(
        id => this.mojoEventTarget_.removeListener(id));
      this.mojoEventTarget_.$.close();
      this.mojoEventTarget_ = null;
    },

    selectShareTarget(targetName) {
      var shareTarget = this.shareTargetNameMap.get(targetName);
      // The attachments to send are only available to the chrome://nearby/
      // page's discoveryManager, so we'll need to access and use it to select
      // the share target and perform the transfer.
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

    onShareTargetDiscovered_(shareTarget) {
      this.shareTargetNameMap.set(shareTarget.name, shareTarget);
    },

    onShareTargetLost_(shareTarget) {
      this.shareTargetNameMap.delete(shareTarget.name);
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