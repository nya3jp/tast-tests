// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

'use strict';

let decodedFrames = [];

async function DecodeFrames(videoURL, width, height, numFrames,
                            hardwareAcceleration) {
  decodedFrames = await decodeVideoInURL(videoURL, numFrames,
                                         hardwareAcceleration);
  TEST.expect(decodedFrames.length == numFrames,
              'The number of decoded frames mismatch: ' + decodedFrames.length);
  for (const frame of decodedFrames) {
    TEST.expect(frame.visibleRect.x == 0 && frame.visibleRect.y == 0,
                'The origin of visible rect is not (0, 0): ' +
                JSON.stringify(frame.visibleRect));
    TEST.expect(
      frame.visibleRect.width == width && frame.visibleRect.height == height,
      'Unexpected width and height of visible rect: ' +
                JSON.stringify(frame.visibleRect));
  }
}

async function GetFrame(index) {
  if (index >= decodedFrames.length) {
    return undefined;
  }

  const frame = decodedFrames[index];
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
