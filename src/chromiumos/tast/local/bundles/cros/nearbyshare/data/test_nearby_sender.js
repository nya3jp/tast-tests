/**
 * @fileoverview Description of this file.
 */
function () {
    return {
        shareTargetNameMap: new Map(),

        confirmationManager: null,

        transferUpdateListener: null,

        currentTransferStatus: null,

        transferUpdateListenerReceiver_: null,

        mojoEventTarget_: null,

        listenerIDs_: null,

        getDiscoveryManager() {
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

            this.getDiscoveryManager()
                .startDiscovery(this.mojoEventTarget_.$.bindNewPipeAndPassRemote())
                .then(response => {
                    if (!response.success) {
                        // TODO(crbug.com/1123934): Show error.
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

        selectShareTargetRes: null,

        selectShareTarget(targetName) {
            var shareTarget = this.shareTargetNameMap.get(targetName);
            if (!shareTarget) {
                this.selectShareTargetRes = "no_share_target";
                return;
            };
            import('./discovery_manager.js')
                .then(m => m.getDiscoveryManager().selectShareTarget(shareTarget.id))
                .then(response => {
                    const {
                        result,
                        transferUpdateListener,
                        confirmationManager
                    } =
                        response;
                    if (result !== nearbyShare.mojom.SelectShareTargetResult.kOk) {
                        this.selectShareTargetRes = "bad_res"
                        return;
                    }

                    this.confirmationManager = confirmationManager;
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
                this.confirmationToken_ = token;
            }
            this.currentTransferStatus = status;
            console.log('token: ', token)
            console.log('currentTransferStatus: ', status)
        },

        accept() {
            this.confirmationManager.accept();
        },

        reject() {
            this.confirmationManager.reject();
        },

        cancel() {
            this.confirmationManager.cancel();
        },
    }
}