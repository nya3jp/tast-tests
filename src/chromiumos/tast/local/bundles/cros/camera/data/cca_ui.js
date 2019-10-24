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
 * @param {function(!AppWindow): undefined} changeState The function to
 *     trigger the state change of the window.
 * @return {!Promise<undefined>} A completion Promise that will be resolved
 *     when the window is in the target state.
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

class LegacyVCDError extends Error {
  /**
   * @param {string=} message
   * @public
   */
  constructor(
      message =
          'Call to unsupported mojo operation on legacy VCD implementation.') {
    super(message);
    this.name = this.constructor.name;
  }
};

/**
 * @typedef {{
 *   width: number,
 *   height: number,
 * }}
 */
var Resolution;

window.Tast = class {
  static isVideoActive() {
    const video = document.querySelector('#preview-video');
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

  /**
   * Returns whether the target HTML element is visible.
   * @param {string} selector Selector for the target element.
   * @return {boolean}
   */
  static isVisible(selector) {
    const element = document.querySelector(selector);
    const style = element && window.getComputedStyle(element);
    return style && style.display !== 'none' && style.visibility !== 'hidden';
  }

  /**
   * Triggers click event on the target HTML element that specified by
   * |selector|. If more than one element matched the selector, it will
   * trigger the first one whose display property is non-null.
   * @param {string} selector Selector for the target element.
   */
  static click(selector) {
    const element =
        Array.from(document.querySelectorAll(selector)).find((element) => {
          const style = window.getComputedStyle(element);
          return style.display !== 'none';
        });
    if (!element) {
      throw new Error('No visible element: ', selector);
    }
    element.click();
  }

  /**
   * Switches to specific camera mode.
   * @param {string} mode The target mode which we expects to switch to.
   * @throws {Error} Throws error if there is no button found for given |mode|.
   */
  static switchMode(mode) {
    try {
      this.click(`.mode-item>input[data-mode="${mode}"]`);
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
    const deviceOperator = await cca.mojo.DeviceOperator.getInstance();
    if (!deviceOperator) {
      return false;
    }
    return deviceOperator.isPortraitModeSupported();
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
    throw new Error(`Expected facing: ${expected}; actual: ${actual};`);
  }

  /**
   * Switcthes the camera device to next available camera.
   * @return {Promise} resolves after preview is active again.
   */
  static switchCamera() {
    this.click('#switch-device');
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
   *     Returns 'unknown' if current device is HALv1 and does not have
   *     configurations.
   */
  static async getFacing() {
    const track = document.querySelector('#preview-video')
                      .srcObject.getVideoTracks()[0];
    const deviceOperator = await cca.mojo.DeviceOperator.getInstance();
    if (!deviceOperator) {
      // This might be a HALv1 device.
      const facing = track.getSettings().facingMode;
      return facing ? facing : 'unknown';
    }
    const facing =
        await deviceOperator.getCameraFacing(track.getSettings().deviceId);
    switch (facing) {
      case cros.mojom.CameraFacing.CAMERA_FACING_FRONT:
        return 'user';
      case cros.mojom.CameraFacing.CAMERA_FACING_BACK:
        return 'environment';
      case cros.mojom.CameraFacing.CAMERA_FACING_EXTERNAL:
        return 'external';
      default:
        throw new Error('Unexpected CameraFacing value: ' + facing);
    }
  }

  /**
   * Gets device id of current active camera device.
   * @return {string} Device id of current active camera.
   * @throws {Error} Failed to get device id from video stream.
   */
  static getDeviceId() {
    const video = document.querySelector('#preview-video');
    if (!video) {
      throw new Error('Cannot find video element.');
    }
    const stream = video.srcObject;
    if (!stream) {
      throw new Error('No MediaStream associate to video.');
    }
    const track = stream.getVideoTracks()[0];
    if (!track) {
      throw new Error('No video track associate to MediaStream.');
    }
    return track.getSettings().deviceId;
  }

  /*
   * Checks if mojo connection could be constructed without error. In this check
   * we only check if the path works and does not check for the correctness of
   * each mojo calls.
   * @param {boolean} shouldSupportDeviceOperator True if the device should
   *     support DeviceOperator.
   * @return {Promise} The promise resolves successfully if the check passes.
   */
  static async checkMojoConnection(shouldSupportDeviceOperator) {
    // Checks if ChromeHelper works. It should work on all devices.
    const chromeHelper = cca.mojo.ChromeHelper.getInstance();
    await chromeHelper.isTabletMode();

    const isDeviceOperatorSupported =
        await cca.mojo.DeviceOperator.isSupported();
    if (shouldSupportDeviceOperator !== isDeviceOperatorSupported) {
      throw new Error(`DeviceOperator support mismatch. Expected: ${
          shouldSupportDeviceOperator} Actual: ${isDeviceOperatorSupported}`);
    }

    // Checks if DeviceOperator works on v3 devices.
    if (isDeviceOperatorSupported) {
      const deviceOperator = await cca.mojo.DeviceOperator.getInstance();
      const devices = (await navigator.mediaDevices.enumerateDevices())
                          .filter(({kind}) => kind === 'videoinput');
      await deviceOperator.getCameraFacing(devices[0].deviceId);
    }
  }

  /**
   * @return {Promise<!cca.mojo.DeviceOperator>}
   * @throws {LegacyVCDError}
   */
  static async getDeviceOperator() {
    if (!await cca.mojo.DeviceOperator.isSupported()) {
      throw new LegacyVCDError();
    }
    return await cca.mojo.DeviceOperator.getInstance();
  }

  /**
   * Gets resolution of preview video.
   * @throws {LegacyVCDError}
   * @return {!Promise<!Array<Resolution>>}
   */
  static getPreviewResolution() {
    const video = document.querySelector('video');
    return {width: video.videoWidth, height: video.videoHeight};
  }

  /**
   * Gets supported photo resolution of current active camera device.
   * @throws {LegacyVCDError}
   * @return {!Promise<!Array<Resolution>>}
   */
  static async getPhotoResolutions() {
    const deviceOperator = await this.getDeviceOperator();
    const deviceId = this.getDeviceId();
    return (await deviceOperator.getPhotoResolutions(deviceId))
        .map(([width, height]) => ({width, height}));
  }

  /**
   * Gets supported video resolution of current active camera device.
   * @throws {LegacyVCDError}
   * @return {!Promise<!Array<Resolution>>}
   */
  static async getVideoResolutions() {
    const deviceOperator = await this.getDeviceOperator();
    const deviceId = this.getDeviceId();
    return (await deviceOperator.getVideoConfigs(deviceId))
        .filter(([, , maxFps]) => maxFps >= 24)
        .map(([width, height]) => ({width, height}));
  }

  /**
   * Toggle expert mode by simulating the activation key press.
   */
  static toggleExpertMode() {
    cca.App.instance_.onKeyPressed_({key: 'Ctrl-Shift-E'});
  }
};
})();
