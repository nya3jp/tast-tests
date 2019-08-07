// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function() {
/**
 * @typedef {chrome.app.window.AppWindow} AppWindow
 */

window.Tast = class {
  /**
   * Changes the state of the current App window.
   * @param {function(!AppWindow): boolean} predicate The function to determine
   *     whether the window is in the target state.
   * @param {function(!AppWindow): !chrome.events.Event} getEventTarget The
   *     function to get the target for adding the event listener.
   * @param {function(!AppWindow): undefined} changeState The function to
   *     trigger the state change of the window.
   * @return {!Promise<undefined>} A completion Promise that will be resolved
   *     when the window is in the target state.
   */
  static changeWindowState(predicate, getEventTarget, changeState) {
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

  static isVideoActive() {
    const video = document.querySelector('video');
    return video && video.srcObject && video.srcObject.active;
  }

  static async restoreWindow() {
    await this.changeWindowState(
        (w) => !w.isMaximized() && !w.isMinimized() && !w.isFullscreen(),
        (w) => w.onRestored, (w) => w.restore());
    // Make sure it's in the foreground even if it's restored from the minimized
    // state.
    chrome.app.window.current().show();
  }

  static minimizeWindow() {
    return this.changeWindowState(
        (w) => w.isMinimized(), (w) => w.onMinimized, (w) => w.minimize());
  }

  static maximizeWindow() {
    return this.changeWindowState(
        (w) => w.isMaximized(), (w) => w.onMaximized, (w) => w.maximize());
  }

  static fullscreenWindow() {
    return this.changeWindowState(
        (w) => w.isFullscreen(), (w) => w.onFullscreened,
        (w) => w.fullscreen());
  }

  /**
   * Returns whether button is visible.
   * @param {string} identifier Identifier for the target button.
   * @return {boolean}
   */
  static isButtonVisible(identifier) {
    const button = document.querySelector(identifier);
    const style = button && window.getComputedStyle(button);
    return style && style.display !== 'none' && style.visibility !== 'hidden';
  }

  static clickButton(identifier) {
    const button = Array.from(document.querySelectorAll(identifier))
                        .find((element) => element.offsetParent);
    if (!button) {
      throw new Error('No visible button: ', identifier);
    }
    button.click();
  }

  static switchMode(mode) {
    try {
      this.clickButton(`.mode-item>input[data-mode="${mode}"]`);
    } catch (e) {
      throw new Error(`Cannot find button for switching to mode ${mode}`);
    }
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
    const video = document.querySelector("#preview-video");
    const videoTrack = video.srcObject.getVideoTracks()[0];
    if (!videoTrack) {
      return false;
    }
    try {
      const imageCapture = new cca.mojo.ImageCapture(videoTrack);
      var capabilities = await imageCapture.getPhotoCapabilities();
    } catch (e) {
      return false;
    }
    return capabilities.supportedEffects &&
        capabilities.supportedEffects.includes(cros.mojom.Effect.PORTRAIT_MODE);
  }

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
    const actual = await this.getFacing();
    if (actual === expected || actual === 'unknown') {
      return;
    }
    throw new Error('Expected facing: ' + expected + '; ' +
        'actual: ' + actual + '; ');
  }

  /**
   * Switcthes the camera device to next available camera.
   * @return {Promise} resolves after preview is active again.
   */
  static switchCamera() {
    this.clickButton('#switch-device');
    return new Promise((resolve, reject) => {
      const interval = setInterval(() => {
        if (cca.state.get('streaming')) {
          clearInterval(interval);
          resolve();
        }
      }, 1000);
    });
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
    let isV1 = false;
    try {
      const imageCapture = new cca.mojo.ImageCapture(track);
      mojoFacing =
          await imageCapture.getCameraFacing(track.getSettings().deviceId);
    } catch (e) {
      // This is HALv1 device.
      isV1 = true;
    }
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

})();
