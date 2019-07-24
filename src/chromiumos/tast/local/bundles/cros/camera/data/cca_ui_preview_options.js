// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function() {

// This class can't be merged into cca_ui.js because it will make the file
// exceed cdp max length and therefore can't be transmitted to dut.
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

  /**
   * Gets device id of current active camera device.
   * @return {string} Device id of current active camera.
   * @throws {Error} Failed to get device id from video stream.
   */
  static getDeviceId() {
    const video = document.querySelector('video');
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
};
})();
