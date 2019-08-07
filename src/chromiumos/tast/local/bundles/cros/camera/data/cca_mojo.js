// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function() {

  // This class can't be merged into cca_ui.js because it will make the file
  // exceed cdp max length and therefore can't be transmitted to dut.
  window.CCAMojo = class {
    // Checks for Mojo connection.
    static async checkMojoConnection() {
      let videoDevices = await navigator.mediaDevices.enumerateDevices().then(
        (devices) => devices.filter((device) => device.kind == 'videoinput'));
      if (videoDevices.length === 0) {
        throw new Error("No video devices detected.");
      }

      // Expects to have no error thrown even if it runs on camera hal v1 stack.
      this.mojoConnector_ = new cca.mojo.MojoConnector();
      let isSupported = await this.mojoConnector_.isDeviceOperationSupported();
      if (isSupported) {
        let deviceOperator = await this.mojoConnector_.getDeviceOperator();
        await deviceOperator.getCameraFacing(videoDevices[0].deviceId);
      }
    }
  };
  })();
