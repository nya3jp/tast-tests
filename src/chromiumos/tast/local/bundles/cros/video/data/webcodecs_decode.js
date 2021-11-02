// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

'use strict';

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
  }
}
