// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

'use strict';

let bitstreamSaver = new BitstreamSaver();

let encoderInputFrames = [];

async function DecodeFrames(videoURL, numFrames) {
  encoderInputFrames = await decodeVideoInURL(videoURL, numFrames);
  TEST.expect(encoderInputFrames.length == numFrames,
              'Decode frames mismatch: ' + encoderInputFrames.length);
  return encoderInputFrames.length == numFrames;
}

async function EncodeAndSave(codec, acceleration, width, height, bitrate,
                             framerate, scalabilityMode) {
  if (scalabilityMode === "") {
    scalabilityMode = undefined;
  }

  let encoder = await CreateEncoder(codec, acceleration, width, height,
                                    bitrate, framerate, bitstreamSaver,
                                    scalabilityMode);
  if (!encoder) {
    TEST.failExit();
    return;
  }

  for (const frame of encoderInputFrames) {
    console.assert(frame, "null frame");
    // A value of false indicates that the User Agent has flexibility to decide
    // whether the frame will be encoded as a key frame.
    encoder.encode(frame, { keyFrame: false });
    frame.close();
  }

  await encoder.flush();
  await encoder.close();

  TEST.expect(
    TEST.numEncodedFrames == encoderInputFrames.length,
    'Encode frames mismatch: ' + TEST.numEncodedFrames);
  TEST.expect(
    TEST.encoderError == 0,
    'Encoding errors occurred during the test');
  TEST.exit();
}
