// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

"use strict";

class TestHarness {
  constructor() {
    this.finished = false;
    this.no_error = true;
    this.logs = [];
    this.num_encoded_frames = 0;
    this.encoder_error = 0;
  }

  complete() {
    return this.finished;
  }

  success() {
    return this.no_error;
  }

  failExit(failure) {
    this.no_error = false;
    this.log(failure);
    this.exit();
  }

  exit() {
    this.finished = true;
  }

  expect(condition, msg) {
    if (!condition) {
      this.no_error = false;
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
    TEST.log(this.bitstreams.length)
    for (let i = 0; i < this.bitstreams.length; i++) {
      // Avoid String.fromCharCode() Maximum call stack size problem by calling
      // it one by one.
      TEST.log(i + ":" + this.bitstreams[i].length);
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
  let encoder_config = {
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
    encoder_config.avc = { format: "annexb" };
  }

  let encoderSupport = await VideoEncoder.isConfigSupported(encoder_config);
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

  const encoder_init = {
    output(chunk, _) {
      if (saver) {
        saver.save(chunk);
      }

      TEST.num_encoded_frames++;
    },
    error(e) {
      TEST.encoder_error++;
      TEST.log("encoder error: " + e);
    },
  };

  let encoder = new VideoEncoder(encoder_init);
  encoder.configure(encoder_config);

  return encoder;
}

// Base class for video frame sources.
class FrameSource {
  constructor() {}

  async getNextFrame() {
    return null;
  }
}

class RawSource extends FrameSource {
  constructor(width, height, framerate, frames) {
    super();

    this.videoFrames = [];
    this.num_read_frames = 0;

    let timespan = 1000 / framerate;
    for (let i = 0; i < frames.length; i++) {
      let frame_buffer_init = {
        format: "I420",
        codedWidth: width,
        codedHeight: height,
        timestamp: timespan * i,
      };

      let data = Uint8Array.from(atob(frames[i]), (c) => c.charCodeAt(0));
      let videoFrame = new VideoFrame(data, frame_buffer_init);
      this.videoFrames.push(videoFrame);
    }
  }

  async getNextFrame() {
    if (this.num_read_frames >= this.videoFrames.length) return undefined;
    return this.videoFrames[this.num_read_frames++];
  }
}
