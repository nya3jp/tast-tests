// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function() {
/**
 * @typedef {chrome.app.window.AppWindow} AppWindow
 */

/**
 * Changes the state of the current App window.
 * @param {function(!AppWindow): boolean} predicate The function to determine
 *     whether the window is in the target state.
 * @param {function(!AppWindow): !chrome.events.Event} getEventTarget The
 *     function to get the target for adding the event listener.
 * @param {function(!AppWindow): undefined} changeState The function to trigger
 *     the state change of the window.
 * @return {!Promise<undefined>} A completion Promise that will be resolved when
 *     the window is in the target state.
 */
function changeWindowState(predicate, getEventTarget, changeState) {
  const win = chrome.app.window.current();
  const eventTarget = getEventTarget(win);
  return new Promise((resolve) => {
    if (predicate(win)) {
      resolve();
      return;
    }
    const onStateChanged = () => {
      eventTarget.removeListener(onStateChanged);
      resolve();
    };
    eventTarget.addListener(onStateChanged);
    changeState(win);
  });
}

window.Tast = class {
  static isVideoActive() {
    const video = document.querySelector('video');
    return video && video.srcObject && video.srcObject.active;
  }

  static async restoreWindow() {
    await changeWindowState(
        (w) => !w.isMaximized() && !w.isMinimized() && !w.isFullscreen(),
        (w) => w.onRestored, (w) => w.restore());
    // Make sure it's in the foreground even if it's restored from the minimized
    // state.
    chrome.app.window.current().show();
  }

  static minimizeWindow() {
    return changeWindowState(
        (w) => w.isMinimized(), (w) => w.onMinimized, (w) => w.minimize());
  }

  static maximizeWindow() {
    return changeWindowState(
        (w) => w.isMaximized(), (w) => w.onMaximized, (w) => w.maximize());
  }

  static fullscreenWindow() {
    return changeWindowState(
        (w) => w.isFullscreen(), (w) => w.onFullscreened,
        (w) => w.fullscreen());
  }
};

window.CCAUICapture = class {
  static clickShutter() {
    const shutter = Array.from(document.querySelectorAll('.shutter'))
                        .find((element) => element.offsetParent);
    if (!shutter) {
      throw new Error('No visible shutter button');
    }
    shutter.click();
  }

  static switchMode(mode) {
    const btn = document.querySelector(`.mode-item>input[data-mode="${mode}"]`);
    if (!btn) {
      throw new Error(`Cannot find button for switching to mode ${mode}`);
    }
    btn.click();
  }

  /**
   * Removes the cached key value pair in chrome.storage.local.
   * @param{Array<string>} keys
   * @return Promise
   */
  static removeCacheData(keys) {
    return new Promise((resolve, reject) => {
      chrome.storage.local.remove(keys, () => {
        if (chrome.runtime.lastError) {
          reject(chrome.runtime.lastError);
        }
        resolve();
      });
    });
  }

  /**
   * Gets whether portrait mode is supported by current active video stream.
   * @return {Promise<boolean>}
   */
  static async isPortraitModeSupported() {
    const mojoConnector = new cca.mojo.MojoConnector();
    const isDeviceOperationSupported =
        await mojoConnector.isDeviceOperationSupported();
    if (!isDeviceOperationSupported) {
      return false;
    }
    const deviceOperator = await mojoConnector.getDeviceOperator();
    return await deviceOperator.isPortraitModeSupported();
  }
};

window.CCAUIMultiCamera = class {
  /**
   * Gets number of cameras.
   * @return {number}
   */
  static async getNumOfCameras() {
    const devices = await navigator.mediaDevices.enumerateDevices();
    return devices.filter((d) => d.kind === 'videoinput').length;
  }

  /**
   * Checks whether facing is as expected. If it's V1 device, accept unknown as
   * correct answer.
   * @param {string} expected Expected facing
   * @return {Promise} The promise resolves successfully if the check passes.
   */
  static async checkFacing(expected) {
    const track = document.querySelector('video').srcObject.getVideoTracks()[0];
    const actual = track.getSettings().facingMode;
    const mojoConnector = new cca.mojo.MojoConnector();
    const isV1 = !await mojoConnector.isDeviceOperationSupported();
    if (expected === actual || (isV1 && (!actual || actual === 'unknown'))) {
      return;
    }
    throw new Error('Expected facing: ' + expected + '; ' +
        'actual: ' + actual + '; ' +
        'isV1: ' + isV1);
  }

  /**
   * Returns whether switch camera button exists.
   * @return {boolean}
   */
  static switchCameraButtonExist() {
    const switchButton = document.querySelector('#switch-device');
    const style = switchButton && window.getComputedStyle(switchButton);
    return style && style.display !== 'none' && style.visibility !== 'hidden';
  }

  /**
   * Switcthes the camera device to next available camera.
   * @return {Promise} resolves after preview is active again.
   */
  static switchCamera() {
    const switchButton = document.querySelector('#switch-device');
    switchButton.click();
    return new Promise((resolve, reject) => {
      const interval = setInterval(() => {
        if (cca.state.get('streaming')) {
          clearInterval(interval);
          resolve();
        }
      }, 1000);
    });
  }
};

window.CCAUIPreviewOptions = class {
  /**
   * Returns whether mirror button exists.
   * @return {boolean}
   */
  static mirrorButtonExist() {
    const mirrorButton = document.querySelector('#toggle-mirror');
    const style = mirrorButton && window.getComputedStyle(mirrorButton);
    return style && style.display !== 'none' && style.visibility !== 'hidden';
  }

  /**
   * Gets facing of current active camera device.
   * @return {string} The facing string 'user', 'environment', 'external'.
   * Returns 'unknown' if current device is HALv1 and does not have
   * configurations.
   */
  static async getFacing() {
    const track = document.querySelector('video').srcObject.getVideoTracks()[0];
    let facing = track.getSettings().facingMode;
    let mojoFacing = null;
    const mojoConnector = new cca.mojo.MojoConnector();
    const isV1 = !await mojoConnector.isDeviceOperationSupported();
    if (mojoFacing !== null) {
      switch (mojoFacing) {
        case cros.mojom.CameraFacing.CAMERA_FACING_FRONT:
          facing = 'user';
          break;
        case cros.mojom.CameraFacing.CAMERA_FACING_BACK:
          facing = 'environment';
          break;
        case cros.mojom.CameraFacing.CAMERA_FACING_EXTERNAL:
          facing = 'external';
          break;
        default:
          facing = null;
      }
    }
    if (isV1 && !facing) {
      facing = 'unknown';
    } else if (!facing) {
      throw new Error('Failed to get facing info');
    }
    return facing;
  }
};

window.CCAMojo = class {
  // Checks for Mojo connection.
  static async checkMojoConnection() {
    let videoDevices = await navigator.mediaDevices.enumerateDevices().then(
      (devices) => devices.filter((device) => device.kind == 'videoinput'));
    if (videoDevices.length === 0) {
      throw new Error("No video devices detected.");
    }

    // Expects that no error would be thrown even if it runs on camera hal v1
    // stack.
    this.mojoConnector_ = new cca.mojo.MojoConnector();
    let isSupported = await this.mojoConnector_.isDeviceOperationSupported();
    if (isSupported) {
      let deviceOperator = await this.mojoConnector_.getDeviceOperator();
      await deviceOperator.getCameraFacing(videoDevices[0].deviceId);
    }
  }
};

})();
