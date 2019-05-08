// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

const DEFAULT_CONSTRAINTS = {audio: true, video: true};

/**
 * @param {string} mimeType
 * @param {number} slice The number of milliseconds to record into each blob
 * @return {Promise<MediaRecorder>}
 */
async function createAndStartMediaRecorder(mimeType = '', slice = 1) {
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
async function createMediaRecorder(mimeType = '') {
  // getUserMedia() might throw errors.
  const stream = await navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS);
  return createMediaRecorderWithStream(stream, mimeType);
}

/**
 * @param {MediaStream} stream
 * @param {string} mimeType
 * @return {Promise<MediaRecorder>}
 */
function createMediaRecorderWithStream(stream, mimeType = '') {
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
  const recorder = await createMediaRecorder();
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
  const recorder = await createAndStartMediaRecorder();
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

/**
 * Tests that when MediaRecorder's start(timeSlice) is called, some data
 * available events are fired containing non empty blob data.
 */
async function testStartWithTimeSlice() {
  const defaultTimeSlice = 100;
  let timeStampCount = 0;

  const recorder = await createAndStartMediaRecorder(
      '', defaultTimeSlice);
  return await new Promise((resolve, reject) => {
    recorder.ondataavailable = (event) => {
      timeStampCount++;
      if (event.data.size <= 0) {
        reject(new Error('Recorder produced empty blob'));
      }

      if (timeStampCount > 10) {
        resolve();
      }
    };
  });
}

/**
 * Tests that when a MediaRecorder's resume() is called, the |state| is
 * 'recording' and a 'resume' event is fired.
 */
async function testResumeAndRecorderState() {
  const recorder = await createAndStartMediaRecorder();
  recorder.pause();
  return await new Promise((resolve, reject) => {
    recorder.onresume = (event) => {
      resolve();
    };
    recorder.resume();
  });
}

/**
 * Tests that MediaRecorder sends data blobs when resume() is called.
 */
async function testResumeAndDataAvailable() {
  const recorder = await createAndStartMediaRecorder();
  recorder.pause();
  return await new Promise((resolve, reject) => {
    recorder.ondataavailable = (event) => {
      if (event.data.size > 0) {
        resolve();
      } else {
        reject(new Error('Recorder produced empty blob'));
      }
    };
    recorder.resume();
  });
}

/**
 * Tests that when paused the recorder will transition |state| to |paused| and
 * then trigger a |pause| event.
 */
async function testPauseAndRecorderState() {
  const recorder = await createAndStartMediaRecorder();
  return await new Promise((resolve, reject) => {
    recorder.onpause = (event) => {
      if (recorder.state === 'paused') {
        resolve();
      } else {
        reject(new Error('Recording state is unexpected: ' + recorder.state));
      }
    };
    recorder.pause();
  });
}

/**
 * Tests that it is possible to stop a paused MediaRecorder and that the |state|
 * becomes 'inactive'.
 */
async function testPauseStopAndRecorderState() {
  const recorder = await createAndStartMediaRecorder();
  recorder.pause();
  recorder.stop();
  if (recorder.state !== 'inactive') {
    throw new Error('Recording state is unexpected: ' + recorder.state);
  }
}

/**
 * Tests that no dataavailable event is fired after MediaRecorder's pause()
 * function is called.
 */
async function testPausePreventsDataavailableFromBeingFired() {
  const recorder = await createAndStartMediaRecorder();
  recorder.pause();
  return await new Promise((resolve, reject) => {
    recorder.ondataavailable = (event) => {
      reject(new Error('Received unexpected data after pause'));
    };
    setTimeout(resolve, 2000);
  });
}

/**
 * Tests that it is not possible to resume an inactive MediaRecorder.
 */
async function testIllegalResumeThrowsDOMError() {
  const recorder = await createMediaRecorder();
  try {
    recorder.resume();
  } catch (e) {
    return;
  }
  throw new Error('Inactive recorder was resumed');
}

/**
 * Tests that MediaRecorder's pause() throws an exception if |state| is not
 * 'recording'.
 */
async function testIllegalPauseThrowsDOMError() {
  const recorder = await createMediaRecorder();
  try {
    recorder.pause();
  } catch (e) {
    return;
  }
  throw new Error('Inactive recorder was paused');
}

/**
 * Tests that MediaRecorder's stop() throws an exception if |state| is not
 * 'recording'.
 */
async function testIllegalStopThrowsDOMError() {
  const recorder = await createMediaRecorder();
  try {
    recorder.stop();
  } catch (e) {
    return;
  }
  throw new Error('Inactive recorder was stopped');
}

/**
 * Tests that MediaRecorder's start() throws an exception if |state| is
 * 'recording'.
 */
async function testIllegalStartInRecordingStateThrowsDOMError() {
  const recorder = await createAndStartMediaRecorder();
  try {
    recorder.start(1);
  } catch (e) {
    return;
  }
  throw new Error('Active recorder was started again');
}

/**
 * Tests that MediaRecorder's start() throws an exception if |state| is
 * 'paused'.
 */
async function testIllegalStartInPausedStateThrowsDOMError() {
  const recorder = await createAndStartMediaRecorder();
  recorder.pause();
  try {
    recorder.start(1);
  } catch (e) {
    return;
  }
  throw new Error(
      'Paused recorder should throw exceptions when calling start()');
}

/**
 * Tests that MediaRecorder's requestData() throws an exception if |state| is
 * 'inactive'.
 */
async function testIllegalRequestDataThrowsDOMError() {
  const recorder = await createMediaRecorder();
  try {
    recorder.requestData();
  } catch (e) {
    return;
  }
  throw new Error(
      'Calling requestdata() in inactive state should throw a DOM Exception');
}

/**
 * Tests that MediaRecorder can record a 2 Channel audio stream.
 */
async function testTwoChannelAudio() {
  const defaultFrequency = 880;
  const context = new AudioContext();
  const oscillator = context.createOscillator();
  oscillator.type = 'sine';
  oscillator.frequency.value = defaultFrequency;
  const dest = context.createMediaStreamDestination();
  dest.channelCount = 2;
  oscillator.connect(dest);
  const recorder = createMediaRecorderWithStream(dest.stream);
  return await new Promise((resolve, reject) => {
    recorder.ondataavailable = (event) => {
      resolve();
    };
    recorder.start(1);
    oscillator.start();
  });
}

/**
 * Tests that MediaRecorder fires an Error Event when the associated MediaStream
 * gets a Track added.
 */
async function testAddingTrackToMediaStreamFiresErrorEvent() {
  const recorder = await createMediaRecorder();
  return await new Promise((resolve, reject) => {
    recorder.start(1);
    recorder.onerror = (event) => {
      resolve();
    };
    // Add a new track, copy of an existing one for simplicity.
    recorder.stream.addTrack(recorder.stream.getTracks()[1].clone());
  });
}

/**
 * Tests that MediaRecorder fires an Error Event when the associated MediaStream
 * gets a Track removed.
 */
async function testRemovingTrackFromMediaStreamFiresErrorEvent() {
  const recorder = await createMediaRecorder();
  return await new Promise((resolve, reject) => {
    recorder.start(1);
    recorder.onerror = (event) => {
      resolve();
    };
    recorder.stream.removeTrack(recorder.stream.getTracks()[1]);
  });
}

