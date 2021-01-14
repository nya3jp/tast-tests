// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/**
 * @enum {number}
 */
let Facing = {
  BACK: 1,
  FRONT: 2,
};

/**
 * @enum {number}
 */
let AspectRatio = {
  AR4X3: 4 / 3,
  AR16X9: 16 / 9,
};

class Preview {
  constructor() {
    /**
     * @const {!HTMLVideoElement}
     */
    this.video =
        /** @type {!HTMLVideoElement} */ (document.querySelector('video'));

    /** @type {?Facing} */
    this.facing = null;

    /** @type {?AspectRatio} */
    this.aspectRatio = null;
  }

  /**
   * @param {!Facing} facing
   * @param {!AspectRatio} aspectRatio
   * @return {!Promise}
   * @throws {!DOMException} OverconstrainedError is thrown on device without
   *     support of target facing or aspect ratio.
   */
  async open(facing, aspectRatio) {
    const constraints = {
      audio: false,
      video: {
        facingMode: facing === Facing.BACK ? 'environment' : 'user',
        aspectRatio,
      },
    };
    const stream = await navigator.mediaDevices.getUserMedia(constraints);
    await new Promise((resolve, reject) => {
      this.video.oncanplay = resolve;
      this.video.onerror = (e) => reject(e.error);
      this.video.srcObject = stream;
    });
    this.video.width = 800;
    this.video.height = 800 / aspectRatio;
    await new Promise((resolve, reject) => {
      this.video.onplaying = resolve;
      this.video.onerror = (e) => reject(e.error);
      this.video.play();
    });
    this.facing = facing;
    this.aspectRatio = aspectRatio;
  }

  /**
   * @public
   */
  close() {
    this.video.pause();
    this.video.srcObject.getVideoTracks()[0].stop();
    this.facing = null;
    this.aspectRatio = null;
  }

  /**
   * @return {!OffscreenCanvas}
   */
  getFrame() {
    const canvas =
        new OffscreenCanvas(this.video.videoWidth, this.video.videoHeight);
    const ctx =
        /** @type {!CanvasRenderingContext2D} */ (canvas.getContext('2d'));
    ctx.drawImage(this.video, 0, 0);
    return canvas;
  }
}

/**
 * @param {number} ms
 * @return {!Promise}
 */
async function sleep(ms) {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

/**
 * @suppress {strictMissingProperties}
 */
window.Tast = class Tast {
  /**
   * @return {!Preview}
   * @private
   */
  static get preview_() {
    if (!window.Tast.preview_instance_) {
      window.Tast.preview_instance_ = new Preview();
    }
    return window.Tast.preview_instance_;
  }

  /**
   * @param {!Facing} facing
   * @param {!AspectRatio} aspectRatio
   * @return {!Promise<!OffscreenCanvas>}
   * @private
   */
  static async getPreviewFrame_(facing, aspectRatio) {
    if (Tast.preview_.facing !== null &&
        (Tast.preview_.facing !== facing ||
         Tast.preview_.aspectRatio !== aspectRatio)) {
      Tast.preview_.close();
    }
    if (Tast.preview_.facing === null) {
      await Tast.preview_.open(facing, aspectRatio);
    }
    return Tast.preview_.getFrame();
  }

  /**
   * Checks the |aspectRatio| camera FOV of |facing| camera is aligned with
   * pattern shown on chart tablet by capturing a frame from camera and
   * verifying all pixels on the frame boundary lying in green area of chart
   * pattern.
   * @param {!Facing} facing
   * @param {!AspectRatio} aspectRatio
   * @return {!Promise<boolean>}
   * @private
   */
  static async checkAlign_(facing, aspectRatio) {
    const frame = await Tast.getPreviewFrame_(facing, aspectRatio);

    const getHue = (r, g, b) => {
      const max = Math.max(r, g, b);
      const min = Math.min(r, g, b);
      const d = max - min;
      if (d === 0) {
        // No hue value e.g. white or black.
        return -1;
      }
      let angle;
      if (max === r) {
        angle = ((g - b) / d + 6) % 6 * 60;
      } else if (max === g) {
        angle = ((b - r) / d + 2) * 60;
      } else {
        angle = ((r - g) / d + 4) * 60;
      }
      return angle;
    };

    const ctx =
        /** @type {!CanvasRenderingContext2D} */ (frame.getContext('2d'));
    const isGreenPixel = (x, y) => {
      const imageData = ctx.getImageData(x, y, 1, 1);
      const [r, g, b] = imageData.data;
      const hue = getHue(r, g, b);
      // 80 <= hue <= 140 according to target green pattern.
      return 80 <= hue && hue <= 140;
    };

    // Check all boundary pixels fall on target pattern.
    for (let x of [0, frame.width - 1]) {
      for (let y = 0; y < frame.height; y++) {
        if (!isGreenPixel(x, y)) {
          return false;
        }
      }
    }
    for (let y of [0, frame.height - 1]) {
      for (let x = 0; x < frame.width; x++) {
        if (!isGreenPixel(x, y)) {
          return false;
        }
      }
    }
    return true;
  }

  /**
   * @param {boolean} passed
   * @param {string} message
   * @private
   */
  static feedbackAlign_(passed, message) {
    document.body.classList.toggle('failed', !passed);
    document.querySelector('.message').textContent = message;
  };

  /**
   * Waits for all sampled frames captured in last N milliseconds from |facing|
   * camera in |aspectRatio| FOV passing alignment check.
   * @param {!Facing} facing
   * @param {!AspectRatio} aspectRatio
   * @param {number} ms
   * @return {!Promise}
   * @private
   */
  static async waitForPassAlignN_(facing, aspectRatio, ms) {
    const aspectRatioName = aspectRatio === AspectRatio.AR4X3 ? '4x3' : '16x9';
    let startTime = null;
    while (true) {
      await sleep(200);
      const currentTime = Date.now();
      if (!await Tast.checkAlign_(facing, aspectRatio)) {
        Tast.feedbackAlign_(false, `Check ${aspectRatioName} align failed`);
        startTime = null;
        continue;
      }
      if (startTime === null) {
        startTime = currentTime;
        Tast.feedbackAlign_(true, `Pass check ${aspectRatioName} align`);
        continue;
      }
      const duration = currentTime - startTime;
      Tast.feedbackAlign_(
          true,
          `Pass check ${aspectRatioName} align ${duration / 1000} seconds`);
      if (duration >= ms) {
        break;
      }
    }
  }

  /**
   * @param {!Facing} facing
   * @return {!Promise}
   */
  static async manualAlign(facing) {
    while (true) {
      await Tast.waitForPassAlignN_(facing, AspectRatio.AR4X3, 5);
      await Tast.waitForPassAlignN_(facing, AspectRatio.AR16X9, 5);

      if (!await Tast.checkAlign_(facing, AspectRatio.AR4X3)) {
        Tast.feedbackAlign_(false, 'Check 4x3 align failed');
        continue;
      }

      if (!await Tast.checkAlign_(facing, AspectRatio.AR16X9)) {
        Tast.feedbackAlign_(false, 'Check 16x9 align failed');
        continue;
      }

      Tast.feedbackAlign_(
          true, `Click settled button once the fixture is settled`);
      const btn = document.querySelector('#settled');
      btn.hidden = false;
      await new Promise((resolve) => {
        btn.onclick = resolve;
      });
      btn.hidden = true;

      if (!await Tast.checkAlign_(facing, AspectRatio.AR4X3)) {
        Tast.feedbackAlign_(false, 'Check 4x3 align failed');
        continue;
      }

      if (!await Tast.checkAlign_(facing, AspectRatio.AR16X9)) {
        Tast.feedbackAlign_(false, 'Check 16x9 align failed');
        continue;
      }
      break;
    }

    Tast.feedbackAlign_(true, 'All passed');
  }
};
