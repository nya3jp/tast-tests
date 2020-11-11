/**
 * @fileoverview Define custom functions to let test scripts to control
 * Nearby Share Receiver through simple API.
 *
 * about cr.define
 * https://source.chromium.org/chromium/chromium/src/+/master:ui/webui/resources/js/cr.js?q=function%5C%20define%5C(
 * nearbyShare.mojom.Receive* interface
 * https://source.chromium.org/chromium/chromium/src/+/master:chrome/browser/ui/webui/nearby_share/nearby_share.mojom?q=interface%5C%20Receive.*
 * the usage in nearby_share_receive_manager.js
 * https://source.chromium.org/chromium/chromium/src/+/master:chrome/browser/resources/settings/chromeos/nearby_share_page/nearby_share_receive_manager.js
 */

// Define the functions for CrOS as a receiver:
// nearbySnippetReceiver.receiveFile
// nearbySnippetReceiver.onHighVisibilityChanged
// nearbySnippetReceiver.onTransferUpdate
let nearbySnippetReceiver = function() {
  /** @private {string} */
  let receiverName_ = 'Receiver';

  /** @private {string} */
  let senderName_ = 'Sender';

  /** @private {number} */
  let turnAroundTimeoutSeconds_ = 300;

  /** @private {number} */
  let timeoutID_ = 0;

  /** @private {?nearbyShare.mojom.ReceiveManagerInterface} */
  let receiveManager_ = null;

  /** @private {?nearbyShare.mojom.ReceiveObserverReceiver} */
  let observerReceiver_ = null;

  /** @private {boolean} */
  let isSecureConnectionAccept_ = false;

  /** @private {boolean} */
  let isFileSent_ = false;

  /** @private {?nearbyShare.mojom.ShareTarget} */
  let currentShareTarget_ = null;

  /** @private {Map!} */
  let receiverResultCallback_ = {
    onLocalConfirmation: () => {
      nearbySnippetEventCache.postEvent('onLocalConfirmation', {});
    },
    onReceivedSuccess: () => {
      nearbySnippetEventCache.postEvent(
          'onReceiveStatus', {'receiveSuccess': true});
    },
    onReceivedFailed: () => {
      nearbySnippetEventCache.postEvent(
          'onReceiveStatus', {'receiveSuccess': false});
    },
    onStop: () => {
      nearbySnippetEventCache.postEvent('onStop', {});
    }
  };

  /**
   * Start Nearby sharing at receiver side with turnaround timeout
   * limitation.
   *
   * @param {string} senderName the name of nearby sharing sender.
   * @param {string} receiverName the name of nearby sharing receiver.
   * @param {number} turnAroundTimeoutSeconds timeout limitation in seconds.
   */
  /* #export */
  function receiveFile(senderName, receiverName, turnAroundTimeoutSeconds) {
    console.log(
        'Starting Nearby sharing at receiver side: \'%s\'.', receiverName);
    receiverName_ = receiverName;
    senderName_ = senderName;
    turnAroundTimeoutSeconds_ = turnAroundTimeoutSeconds;

    timeoutID_ =
        window.setTimeout(timeoutShutdown, turnAroundTimeoutSeconds * 1000);

    observerReceiver_ = new nearbyShare.mojom.ReceiveObserverReceiver(
        /** @type {!nearbyShare.mojom.ReceiveManagerObserverInterface} */ (
            this));
    receiveManager_ = nearbyShare.mojom.ReceiveManager.getRemote();
    receiveManager_.addReceiveObserver(
        observerReceiver_.$.bindNewPipeAndPassRemote());
    receiveManager_.registerForegroundReceiveSurface();
  }

  /** @private */
  function timeoutShutdown() {
    console.log(
        'Turnaround timeout: %d seconds, ' +
            'start to force unregister ReceiveSurface.',
        turnAroundTimeoutSeconds_);
    cancelAndUnRegisterReceiveSurface();
  }

  /** @private */
  function normalShutdown() {
    window.clearTimeout(timeoutID_);
    cancelAndUnRegisterReceiveSurface();
  }

  /** @private */
  function cancelAndUnRegisterReceiveSurface() {
    if (currentShareTarget_ != null && isSecureConnectionAccept_ &&
        !isFileSent_) {
      console.log('Reject receiving file from the sender \'%s\'.', senderName_);
      receiveManager_.reject(currentShareTarget_.id).then((success) => {
        console.log(
            'Receiver "%s" rejected result: \'%d\'.', receiverName_, success);
      });
      currentShareTarget_ = null;
    } else {
      console.log(
          'Skip rejecting of file receiving due to no sender found, ' +
          'no secure connection accepted or file received completely.');
    }

    if (observerReceiver_) {
      observerReceiver_.$.close();
      observerReceiver_ = null;
    }
    receiveManager_.unregisterForegroundReceiveSurface();

    console.log('ReceiveSurface unregistered.');

    receiverResultCallback_.onStop();
  }

  /**
   * Mojo callback when high visibility changes.
   * @param {boolean} inHighVisibility
   */
  function onHighVisibilityChanged(inHighVisibility) {
    console.log('onHighVisibilityChanged', inHighVisibility);
  }

  /**
   * Mojo callback when transfer status changes.
   * @param {!nearbyShare.mojom.ShareTarget} shareTarget
   * @param {!nearbyShare.mojom.TransferMetadata} transferMetadata
   */
  function onTransferUpdate(shareTarget, transferMetadata) {
    const finalStatus = transferMetadata.isFinalStatus;
    console.log('onTransferUpdate() from ReceiveObserver.onTransferUpdate');
    console.log('shareTarget', shareTarget);
    console.log('transferMetadata', transferMetadata);

    if (finalStatus) {
      handleFinalStatus(shareTarget, transferMetadata);
    } else {
      handleOngoingStatus(shareTarget, transferMetadata);
    }
  }

  /**
   * @param {!nearbyShare.mojom.ShareTarget} shareTarget
   * @param {!nearbyShare.mojom.TransferMetadata} transferMetadata
   */
  function handleFinalStatus(shareTarget, transferMetadata) {
    const deviceName = shareTarget.name;
    const status = transferMetadata.status;

    if (deviceName !== senderName_) {
      console.log(
          'handleFinalStatus() skipped due to sender name not matched.');
      return;
    }

    if (isSecureConnectionAccept_ &&
        status === nearbyShare.mojom.TransferStatus.kComplete) {
      isFileSent_ = true;
      console.log(
          'Successfully received a file from sender \'%s\'.', senderName_);
      receiverResultCallback_.onReceivedSuccess();
    } else if (isSecureConnectionAccept_) {
      console.log(
          'Failed to receive a file from sender \'%s\', ' +
              'got \'%d\' status.',
          senderName_, status);
      receiverResultCallback_.onReceivedFailed();
    } else {
      console.log(
          'Failed with \'%d\' status, receiver has not found sender \'%s\' yet.',
          status, senderName_);
    }
    normalShutdown();
  }

  /**
   * @param {!nearbyShare.mojom.ShareTarget} shareTarget
   * @param {!nearbyShare.mojom.TransferMetadata} transferMetadata
   */
  function handleOngoingStatus(shareTarget, transferMetadata) {
    const deviceName = shareTarget.name;
    const status = transferMetadata.status;

    if (deviceName !== senderName_ || isSecureConnectionAccept_) {
      console.log(
          'handleOngoingStatus() skipped due to sender name not ' +
          'matched or secure connection already accepted.');
      return;
    }

    if (status == nearbyShare.mojom.TransferStatus.kAwaitingLocalConfirmation) {
      console.log('Receiver accepted connection from \'%s\'.', senderName_);

      // Record current share target for timeout cancellation.
      currentShareTarget_ = shareTarget;

      receiveManager_.accept(currentShareTarget_.id).then((success) => {
        console.log('Receiver accepte result: \'%d\'.', success);
        if (success) {
          isSecureConnectionAccept_ = true;
          receiverResultCallback_.onLocalConfirmation();
        } else {
          normalShutdown();
        }
      });
    }
  }

  // #cr_define_end
  return {
    receiveFile: receiveFile,
    // Callback functions for nearbyShare.mojom.ReceiveManagerObserverInterface.
    onHighVisibilityChanged: onHighVisibilityChanged,
    onTransferUpdate: onTransferUpdate,
  };
}();
