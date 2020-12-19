/**
 * @fileoverview JavaScript to run on chrome://nearby to control the sending flow.
 */
function () {
  return {
    shareTargetNameMap: new Map(),

    currentTransferStatus: null,

    startDiscoveryRes: null,

    selectShareTargetRes: null,

    confirmationToken: null,

    confirmationManager_: null,

    transferUpdateListenerReceiver_: null,

    mojoEventTarget_: null,

    listenerIDs_: null,

    getDiscoveryManager_() {
      discoveryManager = nearbyShare.mojom.DiscoveryManager.getRemote();
      discoveryManager.onConnectionError.addListener(() => discoveryManager = null);
      return discoveryManager;
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
        .then(response => {
          if (!response.success) {
            // this.startDiscoveryRes = response.result;
            // if (response.result != nearbyShare.mojom.StartDiscoveryResult.kSuccess) {
            return;
          }
        });
    },

    stopDiscovery() {
      if (!this.mojoEventTarget_) {
        return;
      }

      this.shareTargetNameMap.clear();
      this.listenerIds_.forEach(
        id => assert(this.mojoEventTarget_.removeListener(id)));
      this.mojoEventTarget_.$.close();
      this.mojoEventTarget_ = null;
    },

    selectShareTarget(targetName) {
      var shareTarget = this.shareTargetNameMap.get(targetName);
      import('./discovery_manager.js')
        .then(m => m.getDiscoveryManager().selectShareTarget(shareTarget.id))
        .then(response => {
          const {
            result,
            transferUpdateListener,
            confirmationManager
          } =
            response;
          this.selectShareTargetRes = result;
          if (result !== nearbyShare.mojom.SelectShareTargetResult.kOk) {
            return;
          }

          this.confirmationManager_ = confirmationManager;
          this.transferUpdateListenerReceiver_ =
            new nearbyShare.mojom.TransferUpdateListenerReceiver(this);
          this.transferUpdateListenerReceiver_.$.bindHandle(
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