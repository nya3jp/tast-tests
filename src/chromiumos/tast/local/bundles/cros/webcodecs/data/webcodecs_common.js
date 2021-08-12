// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

"use strict";

const kWidth = 1280;
const kHeight = 720;
const kFramerate = 30.0;
const kFrames = 30;
const kKeyFrameInterval = 10;

let localMedia = document.getElementById('localMedia');
let remoteMedia = document.getElementById('remoteMedia');

async function EncodeDecode(codec, encoderAcceleration, decoderAcceleration,
                            sourceType, scalabilityMode = undefined, bitrateMode = 'constant') {
    let encoder_config = {
        codec: codec,
        hardwareAcceleration: encoderAcceleration,
        width: kWidth,
        height: kHeight,
        bitrate: kWidth * kHeight * kFramerate * 0.05,
        framerate : kFramerate,
        scalabilityMode: scalabilityMode,
        bitrateMode: bitrateMode,
        latencyMode: "quality",
    };
    if (codec.startsWith('avc1')) {
        encoder_config.avc = { format : 'annexb' };
    }

    let encoderSupport = await VideoEncoder.isConfigSupported(encoder_config);

    if (!encoderSupport.supported) {
        TEST.log('codec is not supported by ' + (encoderAcceleration == 'require' ? 'hardware' : 'software') + ' encoder: ' + codec);
        TEST.failExit();
        return;
    }

    const decoder_config = {
        codec: codec,
        hardwareAcceleration: decoderAcceleration,
        codedWidth: kWidth,
        codedHeight: kHeight,
    };

    let decoderSupport = await VideoDecoder.isConfigSupported(decoder_config);
    if (!decoderSupport.supported) {
        TEST.log('codec is not supported by ' + (decoderAcceleration == 'require' ? 'hardware' : 'software') + ' decoder: ' + codec);
        TEST.failExit();
        return;
    }


    let num_decoded_frames = 0;
    let decoder_error = 0;
    let decoder = new VideoDecoder({
        output(frame) {
            num_decoded_frames++;
            drawFrameToCanvas(remoteMedia, frame);
            frame.close();
        },
        error(e) {
            decoder_error++;
            TEST.log('decoder error: ' + e)
        }
    });


    let num_encoded_frames = 0;
    let encoder_error = 0;
    const encoder_init = {
        output(chunk, metadata) {
            let config = metadata.decoderConfig;
            if (config) {
                config.hardwareAcceleration = decoderAcceleration;
                decoder.configure(config);
            }
            decoder.decode(chunk);
            num_encoded_frames++;
        },
        error(e) {
            encoder_error++;
            TEST.log('encoder error: ' + e)
        }
    };

    let encoder = new VideoEncoder(encoder_init);
    encoder.configure(encoder_config);
    let source = await createFrameSource(sourceType, kWidth, kHeight);
    for (let i = 0; i < kFrames; i++) {
        let frame = await source.getNextFrame();
        let keyframe = (i % kKeyFrameInterval == 0);
        encoder.encode(frame, { keyFrame: keyframe });
        frame.close();

        await waitForNextFrame();
    }

    await encoder.flush();
    await decoder.flush();

    TEST.expect(
        num_encoded_frames == kFrames,
        'Encode frames mismatch: ' + num_encoded_frames);
    TEST.expect(
        num_decoded_frames == kFrames,
        'Decoded frames mismatch: ' + num_decoded_frames);
    TEST.expect(
        encoder_error == 0,
        'Encoding errors occurred during the test');
    TEST.expect(
        decoder_error == 0,
        'Decoding errors occurred during the test');

    TEST.exit();
}

class TestHarness {
  finished = false;
  no_error = true;
  logs = [];

  constructor() {}

  complete() {
    return this.finished;
  }

  success() {
    return this.no_error;
  }

  failExit(failure) {
    this.no_error = false;
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
    return "\n" + this.logs.join("\n");
  }
};

var TEST = new TestHarness();

// Base class for video frame sources.
class FrameSource {
  constructor() {}

  async getNextFrame() {
    return null;
  }
}

// Source of video frames coming from MediaStreamTrack.
class StreamSource extends FrameSource {
  constructor(track) {
    super();
    this.media_processor = new MediaStreamTrackProcessor(track);
    this.reader = this.media_processor.readable.getReader();
  }

  async getNextFrame() {
    const result = await this.reader.read();
    const frame = result.value;
    return frame;
  }
}

function waitForNextFrame() {
  return new Promise((resolve, _) => {
    window.requestAnimationFrame(resolve);
  });
}

function drawRainbow(ctx, width, height, text) {
  let gradient = ctx.createLinearGradient(0, 0, width, height);
  gradient.addColorStop(0, 'magenta');
  gradient.addColorStop(0.15, 'blue');
  gradient.addColorStop(0.30, 'green');
  gradient.addColorStop(0.50, 'yellow');
  gradient.addColorStop(0.85, 'orange');
  gradient.addColorStop(1.0, 'red');
  ctx.fillStyle = gradient;
  ctx.fillRect(0, 0, width, height);

  ctx.fillStyle = 'black';
  ctx.font = (height / 4) + 'px fantasy';
  ctx.fillText(text, width / 3, height / 2);

  ctx.lineWidth = 20;
  ctx.strokeStyle = 'turquoise';
  ctx.rect(0, 0, width, height);
  ctx.stroke();
}

function createCanvasCaptureSource(width, height, framerate) {
  let canvas = document.createElement('canvas');
  canvas.id = 'canvas-for-capture';
  canvas.width = width;
  canvas.height = height;
  document.body.appendChild(canvas);

  let ctx = canvas.getContext('2d');
  let drawOneFrame = function(time) {
    drawRainbow(ctx, width, height, time.toString());
    window.requestAnimationFrame(drawOneFrame);
  };
  window.requestAnimationFrame(drawOneFrame);

  const stream = canvas.captureStream(framerate);
  const track = stream.getVideoTracks()[0];
  localMedia.srcObject = stream;
  return new StreamSource(track);
}

async function createFrameSource(type, width, height, framerate) {
  switch (type) {
    case 'camera': {
      let constraints = {audio: false, video: {width: width, height: height,
                                               frameRate: {ideal: framerate, max: framerate}}};
      let stream =
          await window.navigator.mediaDevices.getUserMedia(constraints);
      var track = stream.getTracks()[0];
      if (localMedia.srcObject) {
        localMedia.srcObject.getTracks().forEach(track => track.stop());
        localMedia.srcObject = undefined;
      }
      localMedia.srcObject = stream;
      return new StreamSource(track);
    }
    case 'capture':
      return createCanvasCaptureSource(width, height, framerate);
    default:
      TEST.log("unknown source type: " + type);
      return undefined;
  }
}

async function drawFrameToCanvas(canvas, video_frame) {
  canvas.width = video_frame.displayWidth;
  canvas.height = video_frame.displayHeight;
  let context = canvas.getContext("2d");

  createImageBitmap(video_frame).then((toImageBitmap) => {
    context.drawImage(toImageBitmap, 0, 0);
  });
}
