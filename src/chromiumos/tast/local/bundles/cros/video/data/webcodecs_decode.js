// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

'use strict';

class Frames {
  constructor(frames) {
    this.frames = frames;
  }

  length() {
    return this.frames.length;
  }

  async getFrameInfo(index) {
    if (index >= this.frames.length) {
      return undefined;
    }

    const frame = this.frames[index];
    let buffer = new Uint8Array(frame.allocationSize());
    let layouts = await frame.copyTo(buffer);

    // Avoid String.fromCharCode() maximum call stack size problem by calling
    // it one by one.
    let base64Buffer = '';
    for (const c of buffer) {
      base64Buffer += String.fromCharCode(c);
    }

    return {
      frameBuffer: btoa(base64Buffer),
      format: frame.format,
      layouts: layouts
    };
  }
}

async function DecodeFrames(videoURL, width, height, numFrames,
                            hardwareAcceleration) {
  let decodedFrames = await decodeVideoInURL(videoURL, numFrames,
                                         hardwareAcceleration);
  TEST.expect(decodedFrames.length == numFrames,
              'Number of decoded frames mismatch: ' + decodedFrames.length);
  for (const frame of decodedFrames) {
    TEST.expect(frame.visibleRect.x == 0 && frame.visibleRect.y == 0,
                'Origin of visible rect is not (0, 0): ' +
                JSON.stringify(frame.visibleRect));
    TEST.expect(
      frame.visibleRect.width == width && frame.visibleRect.height == height,
      'Unexpected width and height of visible rect: ' +
                JSON.stringify(frame.visibleRect));

    return new Frames(decodedFrames);
  }
}
