// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

"use strict";

class TestHarness {
  constructor() {
    this.finished = false;
    this.noError = true;
    this.logs = [];
    this.numEncodedFrames = 0;
    this.encoderError = 0;
  }

  success() {
    return this.finished && this.noError;
  }

  failExit(failure) {
    this.noError = false;
    this.log(failure);
    this.exit();
  }

  exit() {
    this.finished = true;
  }

  expect(condition, msg) {
    if (!condition) {
      this.noError = false;
      this.log(msg);
    }
  }

  log(msg) {
    this.logs.push(msg);
    console.log(msg);
  }

  getLogs() {
    return '"' + this.logs.join("\n") + '"';
  }
}

let TEST = new TestHarness();

class BitstreamSaver {
  constructor() {
    this.bitstreams = [];
    this.temporalLayerIds = [];
  }

  save(chunk, metadata) {
    let temporalLayerId = metadata.svc?.temporalLayerId;
    if (temporalLayerId !== undefined) {
      this.temporalLayerIds.push(temporalLayerId);
    }

    let buffer = new Uint8Array(chunk.byteLength);
    chunk.copyTo(buffer);
    this.bitstreams.push(buffer);
  }

  getBitstream() {
    let base64Strings = [];
    for (const bitstream of this.bitstreams) {
      // Avoid String.fromCharCode() maximum call stack size problem by calling
      // it one by one.
      let base64Bitstream = '';
      for (const c of bitstream) {
        base64Bitstream += String.fromCharCode(c);
      }
      base64Strings.push(btoa(base64Bitstream));
    }

    return base64Strings;
  }

  getTemporalLayerIds() {
    return this.temporalLayerIds;
  }
}

async function CreateEncoder(
  codec,
  acceleration,
  width,
  height,
  bitrate,
  framerate,
  saver = undefined,
  scalabilityMode = undefined,
  bitrateMode = "constant"
) {
  let encoderConfig = {
    codec: codec,
    hardwareAcceleration: acceleration,
    width: width,
    height: height,
    bitrate: bitrate,
    framerate: framerate,
    scalabilityMode: scalabilityMode,
    bitrateMode: bitrateMode,
    latencyMode: "quality",
  };
  if (codec.startsWith("avc1")) {
    encoderConfig.avc = { format: "annexb" };
  }

  let encoderSupport = await VideoEncoder.isConfigSupported(encoderConfig);
  if (!encoderSupport.supported) {
    TEST.log(
      "codec is not supported by " +
        (acceleration == "prefer-hardware" ? "hardware" : "software") +
        " encoder: " +
        codec
    );
    TEST.failExit();
    return;
  }

  const encoderInit = {
    output(chunk, metadata) {
      if (saver) {
        saver.save(chunk, metadata);
      }

      TEST.numEncodedFrames++;
    },
    error(e) {
      TEST.encoderError++;
      TEST.log("encoder error: " + e);
    },
  };

  let encoder = new VideoEncoder(encoderInit);
  encoder.configure(encoderConfig);

  return encoder;
}

async function createNewFrame(frame) {
  let buffer = new Uint8Array(frame.allocationSize());
  let layout = await frame.copyTo(buffer);

  let bufferParams = {
    format: frame.format,
    // frame.copyTo() sets the width and height to frame.visibleRect by default.
    // Set codedWidth and codedHeight in bufferInit to frame.visibleRect,
    // otherwise `new VideoFrame` will fail if frame.codedWidth and codedHeight
    // are more than layout.stride or the buffer height.
    codedWidth: frame.visibleRect.width,
    codedHeight: frame.visibleRect.height,
    timestamp: frame.timestamp,
    // TODO(crbug.com/1270610): Sets duration.
    // duration: frame.duration,
    layout : layout,
    visibleRect: frame.visibleRect,
    displayWidth: frame.displayWidth,
    displayHeight: frame.displayHeight,
    colorSpace: frame.colorSpace
  };

  try {
    return new VideoFrame(buffer, bufferParams);
  } catch (e) {
    TEST.log("video frame creation error: " + e);
    return null;
  }
}

async function decodeVideoInURL(videoURL, numFrames, hardwareAcceleration) {
  let demuxer = new MP4Demuxer(videoURL);
  let videoFrames = [];
  let numDecodedFrames = 0;
  let decoder = new VideoDecoder({
    output(frame) {
      createNewFrame(frame).then(newFrame => {
        // newFrame may not be queued in order of calling output() because of
        // createNewFrame execution time.
        numDecodedFrames++;
        if (newFrame !== null)
          videoFrames.push(newFrame);
        frame.close();
      });
    },
    error(e){
      TEST.log("decoder error: " + e);
    }
  });

  let config = await demuxer.getConfig();
  config.hardwareAcceleration = hardwareAcceleration;
  decoder.configure(config);

  let numChunks = 0;
  demuxer.start((chunk) => {
    decoder.decode(chunk);
    numChunks++;
    if (numChunks == numFrames) {
      decoder.flush();
    }
  });

  return new Promise(async(resolve, reject) => {
    (function waitForDecoded(){
      if (numDecodedFrames == numFrames) {
        decoder.close();
        // Sort videoFrames in order of timestamps.
        videoFrames.sort(function(fa, fb) {
          return fa.timestamp - fb.timestamp;
        });
        return resolve(videoFrames);
      }
      setTimeout(waitForDecoded, 20);
    })();
  });
}
