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
   * Null if current device is HALv1 and does not have configurations.
   */
  static async getFacing() {
    const track = document.querySelector('video').srcObject.getVideoTracks()[0];
    let facing = track.getSettings().facingMode;
    let mojoFacing = null;
    try {
      const imageCapture = new cca.mojo.ImageCapture(track);
      mojoFacing =
          await imageCapture.getCameraFacing(track.getSettings().deviceId);
    } catch (e) {
      // This is HALv1 device.
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
    return facing;
  }
};
})();
