// Copyright 2021 The ChromiumOS Authors
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

/**
 * @param {!AspectRatio} aspectRatio
 * @return {string}
 */
function getAspectRatioName(aspectRatio) {
  if (aspectRatio === AspectRatio.AR4X3) {
    return '4x3';
  }
  return '16x9';
}
let cvIsReady = false;
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

class AlignTimeoutError extends Error {
  /**
   * @param {!Facing} facing
   * @param {!AspectRatio} aspectRatio
   */
  constructor(facing, aspectRatio, timeout) {
    super(`Can't align ${
        facing === Facing.BACK ? 'back' : 'front'} facing camera with ${
        getAspectRatioName(aspectRatio)} aspectRatio within ${timeout} ms`);
    this.name = this.constructor.name;

    /**
     * @const {!AspectRatio}
     */
    this.aspectRatio = aspectRatio;
  }
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
    if (!cvIsReady){
      cv = await cv;
      cvIsReady = true;
    }
    const ctx =
    /** @type {!CanvasRenderingContext2D} */ (frame.getContext('2d'));
    const imageData = ctx.getImageData(0, 0, frame.width, frame.height);
    document.getElementById("debug").innerHTML = "Debug"
    return PatternChecker.checkAlign(pattern_img,imageData,wrapImg,0.4);

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
  static async checkAlign_2(facing, aspectRatio) {

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
      // 60 <= hue <= 140 according to target green pattern.
      return 60 <= hue && hue <= 140;
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
   * Waits for all sampled frames captured in last |passMs| milliseconds from
   * |facing| camera in |aspectRatio| FOV passing alignment check.
   * @param {!Facing} facing
   * @param {!AspectRatio} aspectRatio
   * @param {number} passMs
   * @param {number=} timeoutMs Timeout for wait checking criteria pass.
   * @return {!Promise}
   * @private
   */
  static async waitForPassAlignN_(
      facing, aspectRatio, passMs, timeoutMs = Infinity) {
    const aspectRatioName = getAspectRatioName(aspectRatio);
    let startCheckTime = Date.now();
    let startPassTime = null;
    while (true) {
      await sleep(200);
      const currentTime = Date.now();
      if (currentTime - startCheckTime > timeoutMs) {
        throw new AlignTimeoutError(facing, aspectRatio, timeoutMs);
      }
      if (!await Tast.checkAlign_(facing, aspectRatio)) {
        Tast.feedbackAlign_(false, `Check ${aspectRatioName} align failed`);
        startPassTime = null;
        continue;
      }
      if (startPassTime === null) {
        startPassTime = currentTime;
        Tast.feedbackAlign_(true, `Pass check ${aspectRatioName} align`);
        continue;
      }
      const duration = currentTime - startPassTime;
      Tast.feedbackAlign_(
          true,
          `Pass check ${aspectRatioName} align ${duration / 1000} seconds`);
      if (duration >= passMs) {
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
      await Tast.waitForPassAlignN_(facing, AspectRatio.AR4X3, 5000);
      await Tast.waitForPassAlignN_(facing, AspectRatio.AR16X9, 5000);

      Tast.feedbackAlign_(
          true, `Click settled button once the fixture is settled`);
      const btn = document.querySelector('#settled');
      btn.hidden = false;
      await new Promise((resolve) => {
        btn.onclick = resolve;
      });
      btn.hidden = true;

      try {
        // Check the regression test criteria can also pass after finishing
        // manual alignment.
        await Tast.checkRegression_(facing);
      } catch (e) {
        if (e instanceof AlignTimeoutError) {
          const aspectRatioName = getAspectRatioName(e.aspectRatio);
          Tast.feedbackAlign_(false, `Check ${aspectRatioName} align failed`);
          continue;
        }
        throw e;
      }
      break;
    }
  }

  /**
   * @param {!Facing} facing
   * @return {!Promise}
   */
  static async checkRegression_(facing) {
    await Tast.waitForPassAlignN_(facing, AspectRatio.AR4X3, 5000, 15000);
    await Tast.waitForPassAlignN_(facing, AspectRatio.AR16X9, 5000, 15000);
    Tast.feedbackAlign_(true, 'All passed');
  }

  /**
   * Saves frame to download folder.
   * @param {!Facing} facing
   * @param {!AspectRatio} aspectRatio
   */
  static async savePreviewFrame_(facing, aspectRatio) {
    const frame = await Tast.getPreviewFrame_(facing, aspectRatio);
    const blob = await frame.convertToBlob({type: 'image/png'});
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'frame.png';
    document.body.appendChild(a);
    a.click();
    // Delay for waiting frame saved.
    await sleep(1000);
  }

  /**
   * @param {!Facing} facing
   * @return {!Promise}
   */
  static async checkRegression(facing) {
    try {
      await Tast.checkRegression_(facing);
    } catch (e) {
      if (e instanceof AlignTimeoutError) {
        await Tast.savePreviewFrame_(facing, e.aspectRatio);
      }
      throw e;
    }
  }
};
