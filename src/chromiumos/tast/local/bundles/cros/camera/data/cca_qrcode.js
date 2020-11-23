// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/**
 * Loads an Image from |src|.
 * @param {string} src
 * @return {!Promise<!HTMLImageElement>}
 */
const loadImage = (src) => {
  const image = new Image();
  return new Promise((resolve, reject) => {
    image.onload = () => {
      resolve(image);
    };
    image.onerror = () => {
      reject(new Error(`Failed to create image from ${src}`));
    };
    image.src = src;
  });
};

/**
 * Scales and draws the image on canvas. If the aspect ratio does not match,
 * the image would be cropped from the center.
 * @param {!HTMLCanvasElement} canvas
 * @param {!HTMLImageElement} image
 * @param {number} width
 * @param {number} height
 */
const drawOnCanvas = (canvas, image, width, height) => {
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext('2d');
  ctx.clearRect(0, 0, width, height);
  // High quality smoothing would increase the success rate significantly.
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = 'high';
  const ratio = Math.min(image.width / width, image.height / height);
  const sw = ratio * width;
  const sh = ratio * height;
  const sx = (image.width - sw) / 2;
  const sy = (image.height - sh) / 2;
  ctx.drawImage(image, sx, sy, sw, sh, 0, 0, width, height);
};

/**
 * Checks whether the value in |detectedCodes| matches |expectedCode|.
 * @param {!Array<!{rawValue: string}>} detectedCodes
 * @param {string} expectedCode
 * @throws {!Error}
 */
const checkCode = (detectedCodes, expectedCode) => {
  if (detectedCodes.length !== 1) {
    throw new Error(`Expect exactly 1 code, got ${detectedCodes.length}`);
  }
  const value = detectedCodes[0].rawValue;
  if (value !== expectedCode) {
    throw new Error(`Expect code ${expectedCode}, got ${value}`);
  }
};

// Namespace for calling from Tast.
window.Tast = {
  /**
   * Scans the image with the given configuration.
   * @param {string} imageUrl
   * @param {number} width
   * @param {number} height
   * @param {string} expectedCode
   * @param {number} warmupTimes
   * @param {number} times
   * @return {!Promise<number>} The average detection time in ms.
   */
  async scan(imageUrl, width, height, expectedCode, warmupTimes, times) {
    const canvas = document.querySelector('canvas');
    const image = await loadImage(imageUrl);
    drawOnCanvas(canvas, image, width, height);
    const detector = new BarcodeDetector({formats: ['qr_code']});

    for (let i = 0; i < warmupTimes; i++) {
      const detectedCodes = await detector.detect(canvas);
      checkCode(detectedCodes, expectedCode);
    }

    const startTime = performance.now();
    for (let i = 0; i < times; i++) {
      const detectedCodes = await detector.detect(canvas);
      checkCode(detectedCodes, expectedCode);
    }
    const elapsedTime = performance.now() - startTime;
    const avgTime = elapsedTime / times;
    console.log(
        `Scan ${imageUrl} in ${width}x${height} ${times} time(s),`,
        `average time = ${avgTime.toFixed(2)}ms`);
    return avgTime;
  }
};
