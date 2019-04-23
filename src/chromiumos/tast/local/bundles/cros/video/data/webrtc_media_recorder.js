// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

const DEFAULT_CONSTRAINTS = {audio: true, video: true};
const DEFAULT_RECORDER_MIME_TYPE = '';

/**
 * @param {string} mimeType
 * @param {number} slice The number of milliseconds to record into each blob
 * @return {Promise<MediaRecorder>}
 */
async function createAndStartMediaRecorder(mimeType, slice = 1) {
  const recorder = await createMediaRecorder(mimeType);

  document.getElementById('video').srcObject = recorder.stream;
  recorder.start(slice);
  console.log('Recorder started with time slice', slice);
  return recorder;
}

/**
 * @param {string} mimeType
 * @return {Promise<MediaRecorder>}
 */
async function createMediaRecorder(mimeType) {
  // getUserMedia() might throw errors.
  const stream = await navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS);
  const recorder = new MediaRecorder(stream, {mimeType});
  console.log('Recorder object created.');
  return recorder;
}

/**
 * Tests that the MediaRecorder's start() function will cause the |state| to be
 * 'recording' and that a 'start' event is fired.
 * @return {Promise}
 */
async function testStartAndRecorderState() {
  const recorder = await createMediaRecorder(DEFAULT_RECORDER_MIME_TYPE);
  return await new Promise((resolve, reject) => {
    recorder.onstart = (event) => {
      if (recorder.state === 'recording') {
        resolve();
      } else {
        reject(new Error('Recording state is unexpected: ' + recorder.state));
      }
    };
    recorder.start(1);
  });
}

/**
 * Tests that the MediaRecorder's stop() function will effectively cause the
 * |state| to be 'inactive' and that a 'stop' event is fired.
 * @return {Promise}
 */
async function testStartStopAndRecorderState() {
  const recorder = await createAndStartMediaRecorder(
      DEFAULT_RECORDER_MIME_TYPE);
  return await new Promise((resolve, reject) => {
    recorder.onstop = (event) => {
      if (recorder.state === 'inactive') {
        resolve();
      } else {
        reject(new Error('Recording state is unexpected: ' + recorder.state));
      }
    };
    recorder.stop();
  });
}

/**
 * Tests that when MediaRecorder's start() function is called, some data is
 * made available by media recorder via dataavailable events, containing non
 * empty blob data.
 * @param {string} type The MIME type string
 * @return {Promise}
 */
async function testStartAndDataAvailable(type) {
  const recorder = await createAndStartMediaRecorder(type);
  return await new Promise((resolve, reject) => {
    recorder.ondataavailable = (event) => {
      if (event.data.size > 0) {
        resolve();
      } else {
        reject(new Error('Recorder produced empty blob'));
      }
    };
  });
}

