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

  complete() {
    return this.finished;
  }

  success() {
    return this.noError;
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
  }

  save(chunk) {
    let buffer = new Uint8Array(chunk.byteLength);
    chunk.copyTo(buffer);
    this.bitstreams.push(buffer);
  }

  getBitstream() {
    let base64Strings = [];
    for (let i = 0; i < this.bitstreams.length; i++) {
      // Avoid String.fromCharCode() Maximum call stack size problem by calling
      // it one by one.
      let bitstream = '';
      for (let j = 0; j < this.bitstreams[i].length; j++) {
        bitstream += String.fromCharCode(this.bitstreams[i][j]);
      }
      base64Strings.push(btoa(bitstream));
    }

    return base64Strings;
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
    output(chunk, _) {
      if (saver) {
        saver.save(chunk);
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

// Base class for video frame sources.
class FrameSource {
  constructor() {}

  async getNextFrame() {
    return null;
  }
}

class VideoSource extends FrameSource {
  constructor(videoFrames) {
    super();
    this.videoFrames = videoFrames;
    this.numReadFrames = 0;
  }

  async getNextFrame() {
    if (this.numReadFrames >= this.videoFrames.length) return undefined;
    return this.videoFrames[this.numReadFrames++];
  }
}

async function createVideoSource(videoURL, framerate, numFrames) {
  let demuxer = new MP4Demuxer(videoURL);
  let videoFrames = [];
  let decoder = new VideoDecoder({
    output(frame) {
      videoFrames.push(frame);
    },
    error(e){
      TEST.log("decoder error: " + e);
    }
  });

  let config = await demuxer.getConfig();
  config.hardwareAcceleration = "prefer-software";
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
      if (videoFrames.length == numFrames) {
        decoder.close();
        return resolve(new VideoSource(videoFrames));
      }
      setTimeout(waitForDecoded, 20);
    })();
  });
}
