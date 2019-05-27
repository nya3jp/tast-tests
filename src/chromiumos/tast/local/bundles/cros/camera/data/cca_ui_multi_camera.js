// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function() {

// This class can't be merged into cca_ui.js because it will make the file
// exceed cdp max length and therefore can't be transmitted to dut.
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
    let isV1 = false;
    try {
      const imageCapture = new cca.mojo.ImageCapture(track);
      await imageCapture.getPhotoCapabilities();
    } catch (e) {
      isV1 = true;
    }
    if (expected === actual || (isV1 && actual === 'unknown')) {
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

  /**
   * Toggles the grid option button.
   * @return {Promise<boolean>} Whether grid is enabled after toggling
   */
  static toggleGrid() {
    const prev = cca.state.get('grid');
    const gridOption = document.querySelector('#toggle-grid');
    gridOption.click();
    return new Promise((resolve, reject) => {
      const interval = setInterval(() => {
        if (cca.state.get('grid') !== prev) {
          clearInterval(interval);
          resolve(cca.state.get('grid'));
        }
      }, 1000);
    });
  }

};
})();
