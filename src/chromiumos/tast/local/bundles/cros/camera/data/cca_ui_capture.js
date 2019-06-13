// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function() {

// This class can't be merged into cca_ui.js because it will make the file
// exceed cdp max length and therefore can't be transmitted to dut.
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
};
})();
