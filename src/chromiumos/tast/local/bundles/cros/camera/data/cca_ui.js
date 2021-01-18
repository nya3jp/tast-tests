// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(async function() {

/**
 * Imports js modules from CCA source.
 * @param {string} path Import path related to CCA js directory.
 * @return {!Promise<!Module>} Resolves to module in cca.
 */
function ccaImport(path) {
  return import(`/js/${path}`);
}

const state = await ccaImport('state.js');
const {DeviceOperator} = await ccaImport('mojo/device_operator.js');
const {ChromeHelper} = await ccaImport('mojo/chrome_helper.js');
const {Facing} = await ccaImport('type.js');
const {browserProxy} = await ccaImport('browser_proxy/browser_proxy.js');
const {windowController} =
    await ccaImport('window_controller/window_controller.js');

/**
 * @typedef {chrome.app.window.AppWindow} AppWindow
 */

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

// Note: use a named class declaration and refer the class instance via the
// name here, instead of using anonymous class and referring it via "this",
// in order to make it simpler to call methods via Chrome Devtools Protocol.
window.Tast = class Tast {
  static get previewVideo() {
    return document.querySelector('#preview-video');
  }

  static getState(s) {
    return state.get(s);
  }

  static isVideoActive() {
    const video = Tast.previewVideo;
    return video && video.srcObject && video.srcObject.active;
  }

  static isMinimized() {
    return windowController.isMinimized();
  }

  static restoreWindow() {
    return windowController.restore();
  }

  static minimizeWindow() {
    return windowController.minimize();
  }

  static maximizeWindow() {
    return windowController.maximize();
  }

  static fullscreenWindow() {
    return windowController.fullscreen();
  }

  static focusWindow() {
    return windowController.focus();
  }

  /**
   * @return {string}
   */
  static getScreenOrientation() {
    return window.screen.orientation.type;
  }

  /**
   * Returns whether the target HTML element is visible.
   * @param {string} selector Selector for the target element.
   * @return {boolean}
   */
  static isVisible(selector) {
    const element = document.querySelector(selector);
    const style = element && window.getComputedStyle(element);
    const isZeroSize = (element) => {
      const {width, height} = element.getBoundingClientRect();
      return width === 0 && height === 0;
    };
    return style && style.display !== 'none' && style.visibility !== 'hidden' &&
        !isZeroSize(element);
  }

  /**
   * Returns whether the target HTML element is exists.
   * @param {string} selector Selector for the target element.
   * @return {boolean}
   */
  static isExist(selector) {
    return document.querySelector(selector) !== null;
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
    Tast.click(`.mode-item>input[data-mode="${mode}"]`);
  }

  /**
   * Removes all the cached data in chrome.storage.local.
   * @return {Promise}
   */
  static removeCacheData() {
    return browserProxy.localStorageClear();
  }

  /**
   * Gets whether portrait mode is supported by current active video stream.
   * @return {Promise<boolean>}
   */
  static async isPortraitModeSupported() {
    const deviceOperator = await DeviceOperator.getInstance();
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
    const actual = await Tast.getFacing();
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
    Tast.click('#switch-device');
    return new Promise((resolve, reject) => {
      const interval = setInterval(() => {
        if (state.get(state.State.STREAMING)) {
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
    const track = Tast.previewVideo.srcObject.getVideoTracks()[0];
    const deviceOperator = await DeviceOperator.getInstance();
    if (!deviceOperator) {
      // This might be a HALv1 device.
      const facing = track.getSettings().facingMode;
      return facing ? facing : 'unknown';
    }
    const facing =
        await deviceOperator.getCameraFacing(track.getSettings().deviceId);
    switch (facing) {
      case Facing.USER:
      case Facing.ENVIRONMENT:
      case Facing.EXTERNAL:
        return facing;
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
    const video = Tast.previewVideo;
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
    const chromeHelper = ChromeHelper.getInstance();
    await chromeHelper.isTabletMode();

    const isDeviceOperatorSupported = await DeviceOperator.isSupported();
    if (shouldSupportDeviceOperator !== isDeviceOperatorSupported) {
      throw new Error(`DeviceOperator support mismatch. Expected: ${
          shouldSupportDeviceOperator} Actual: ${isDeviceOperatorSupported}`);
    }

    // Checks if DeviceOperator works on v3 devices.
    if (isDeviceOperatorSupported) {
      const deviceOperator = await DeviceOperator.getInstance();
      const devices = (await navigator.mediaDevices.enumerateDevices())
                          .filter(({kind}) => kind === 'videoinput');
      await deviceOperator.getCameraFacing(devices[0].deviceId);
    }
  }

  /**
   * @return {Promise<!DeviceOperator>}
   * @throws {LegacyVCDError}
   */
  static async getDeviceOperator() {
    if (!await DeviceOperator.isSupported()) {
      throw new LegacyVCDError();
    }
    return await DeviceOperator.getInstance();
  }

  /**
   * Gets resolution of preview video.
   * @throws {LegacyVCDError}
   * @return {!Promise<!Array<Resolution>>}
   */
  static getPreviewResolution() {
    const video = Tast.previewVideo;
    return {width: video.videoWidth, height: video.videoHeight};
  }

  /**
   * Gets supported photo resolution of current active camera device.
   * @throws {LegacyVCDError}
   * @return {!Promise<!Array<Resolution>>}
   */
  static async getPhotoResolutions() {
    const deviceOperator = await Tast.getDeviceOperator();
    const deviceId = Tast.getDeviceId();
    return await deviceOperator.getPhotoResolutions(deviceId);
  }

  /**
   * Gets supported video resolution of current active camera device.
   * @throws {LegacyVCDError}
   * @return {!Promise<!Array<Resolution>>}
   */
  static async getVideoResolutions() {
    const deviceOperator = await Tast.getDeviceOperator();
    const deviceId = Tast.getDeviceId();
    return (await deviceOperator.getVideoConfigs(deviceId))
        .filter(({maxFps}) => maxFps >= 24)
        .map(({width, height}) => ({width, height}));
  }

  /**
   * Toggle expert mode by simulating the activation key press.
   */
  static toggleExpertMode() {
    document.body.dispatchEvent(new KeyboardEvent(
        'keydown', {ctrlKey: true, shiftKey: true, key: 'E'}));
  }

  /**
   * Sets observer of the configuration process.
   * @return {!Promise} Promise resolved/rejected after configuration finished.
   */
  static waitNextConfiguration() {
    const CAMERA_CONFIGURING = state.State.CAMERA_CONFIGURING;
    if (state.get(CAMERA_CONFIGURING)) {
      throw new Error('Already in configuring state');
    }

    return new Promise((resolve, reject) => {
      let activated = false;
      const observer = (value) => {
        if (activated === value) {
          state.removeObserver(CAMERA_CONFIGURING, observer);
          reject(new Error(
              `State ${CAMERA_CONFIGURING} assertion failed,` +
              `expecting ${!activated} got ${value}`));
          return;
        }
        if (value) {
          activated = true;
          return;
        }
        state.removeObserver(CAMERA_CONFIGURING, observer);
        resolve();
      }
      state.addObserver(CAMERA_CONFIGURING, observer);
    });
  }

  /**
   * Observes state change event for |name| state changing from |!expected| to
   * |expected|.
   * @param {string} name
   * @param {boolean} expected
   * @return {!Promise<number>} Promise resolved to the millisecond unix
   *     timestamp of when the change happen.
   */
  static async observeStateChange(name, expected) {
    const s = state.assertState(name);
    if (state.get(s) !== !expected) {
      throw new Error(`The current "${s}" state is not ${!expected}`);
    }
    return new Promise((resolve, reject) => {
      const onChange = (changed) => {
        state.removeObserver(s, onChange);
        if (changed !== expected) {
          reject(new Error(`The changed "${s}" state is not ${expected}`));
        }
        resolve(Date.now());
      };
      state.addObserver(s, onChange);
    });
  }
};
})();
