// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
 * @param {!Facing} facing
 * @return {!Promise}
 */
const manualAlign = async (facing) => {
  class Preview {
    constructor() {
      /** @const {!HTMLVideoElement} */
      this.video = document.querySelector('video');
      /** @type {?Facing} */
      this.facing = null;
      /** @type {?AspectRatio} */
      this.aspectRatio = null;
    }

    /**
     * @param {!Facing} facing
     * @param {!AspectRatio} aspectRatio
     * @return {!Promise}
     * @throws {DOMException} OverconstrainedError is thrown on device without
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
        this.video.addEventListener('canplay', resolve);
        this.video.addEventListener('error', reject);
        this.video.srcObject = stream;
      });
      this.video.width = 800;
      this.video.height = 800 / aspectRatio;
      await new Promise((resolve, reject) => {
        this.video.addEventListener('playing', resolve);
        this.video.addEventListener('error', reject);
        this.video.play();
      });
      this.facing = facing;
      this.aspectRatio = aspectRatio;
    }

    async close() {
      this.video.pause();
      const track = this.video.srcObject.getVideoTracks()[0];
      track.stop();
      this.facing = null;
      this.aspectRatio = null;
    }

    /**
     * @return {!HTMLCanvasElement}
     */
    async getFrame() {
      const canvas =
          new OffscreenCanvas(this.video.videoWidth, this.video.videoHeight);
      const ctx = canvas.getContext('2d');
      ctx.drawImage(this.video, 0, 0);
      return canvas;
    }
  }
  const preview = new Preview();

  const sleep = async (ms) => {
    return new Promise((resolve) => {
      setTimeout(resolve, ms);
    });
  };

  /**
   * @param {boolean} passed
   * @param {string} message
   */
  const feedbackAlign = (passed, message) => {
    document.body.classList.toggle('failed', !passed);
    document.querySelector('.message').textContent = message;
  };

  /**
   * @param {!HTMLCanvasElement} frame
   * @return {boolean}
   */
  const checkFrame = (frame) => {
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
        angle = (g - b) / d % 6 * 60;
      } else if (max === g) {
        angle = ((b - r) / d + 2) * 60;
      } else {
        angle = ((r - g) / d + 4) * 60;
      }
      return (angle % 360 + 360) % 360;
    };

    const ctx = frame.getContext('2d');
    const checkPixel = (x, y) => {
      const imageData = ctx.getImageData(x, y, 1, 1);
      const [r, g, b] = imageData.data
      const hue = getHue(r, g, b);
      // 80 <= hue <= 140 according to target green pattern.
      return 80 <= hue && hue <= 140;
    };

    // Check all boundary pixels fall on target pattern.
    for (let x of [0, frame.width - 1]) {
      for (let y = 0; y < frame.height; y++) {
        if (!checkPixel(x, y)) {
          return false;
        }
      }
    }
    for (let y of [0, frame.height - 1]) {
      for (let x = 0; x < frame.width; x++) {
        if (!checkPixel(x, y)) {
          return false;
        }
      }
    }
    return true;
  };

  /**
   * @param {!Facing} facing
   * @param {!AspectRatio} aspectRatio
   * @return {!Promise<!HTMLCanvasElement>}
   */
  const getPreviewFrame =
      async (facing, aspectRatio) => {
    if (preview.facing !== null &&
        (preview.facing !== facing || preview.aspectRatio !== aspectRatio)) {
      await preview.close();
    }
    if (preview.facing === null) {
      await preview.open(facing, aspectRatio);
    }
    return preview.getFrame();
  }

  /**
   * @param {!Facing} facing
   * @param {!AspectRatio} aspectRatio
   * @return {!Promise<boolean>}
   */
  const checkAlign = async (facing, aspectRatio) => {
    const frame = await getPreviewFrame(facing, aspectRatio);
    return checkFrame(frame);
  };

  /**
   * Checks alignment until it pass N times in a row.
   * @param {!Facing} facing
   * @param {!AspectRatio} aspectRatio
   * @param {number} times
   * @return {!Promise}
   */
  const passAlignN = async (facing, aspectRatio, times) => {
    const aspectRatioName = aspectRatio === AspectRatio.AR4X3 ? '4x3' : '16x9';
    let i = 1;
    while (i <= 5) {
      // TODO(b/166370953): Improve the check response time < 1 seconds.
      await sleep(1000);
      if (!await checkAlign(facing, aspectRatio)) {
        feedbackAlign(false, `Check ${aspectRatioName} align failed`);
        i = 1;
        continue
      }
      feedbackAlign(true, `Pass check ${aspectRatioName} align ${i} times`);
      i++;
    }
  };

  while (true) {
    await passAlignN(facing, AspectRatio.AR4X3, 5);
    await passAlignN(facing, AspectRatio.AR16X9, 5);

    if (!await checkAlign(facing, AspectRatio.AR4X3)) {
      feedbackAlign(false, 'Check 4x3 align failed');
      continue;
    }

    if (!await checkAlign(facing, AspectRatio.AR16X9)) {
      feedbackAlign(false, 'Check 16x9 align failed');
      continue;
    }

    feedbackAlign(true, 'Wait 5 seconds for fixture settling');
    await sleep(5000);

    if (!await checkAlign(facing, AspectRatio.AR4X3)) {
      feedbackAlign(false, 'Check 4x3 align failed');
      continue;
    }

    if (!await checkAlign(facing, AspectRatio.AR16X9)) {
      feedbackAlign(false, 'Check 16x9 align failed');
      continue;
    }
    break;
  }

  feedbackAlign(true, 'All passed');
};
