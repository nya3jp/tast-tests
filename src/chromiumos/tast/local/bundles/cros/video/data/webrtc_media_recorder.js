// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

let pageLoaded = false;
let testDone = false;
let result;
const DEFAULT_CONSTRAINTS = {audio: true, video: true};
const DEFAULT_RECORDER_MIME_TYPE = '';

/**
 * Waits for a condition becomes true and periodically logs messages.
 * @param {string} description
 * @param {function():boolean} predicate
 * @return {Promise}
 */
function waitFor(description, predicate) {
  return new Promise((resolve, reject) => {
    let pivotTime = new Date();
    console.log('Waiting for', description.toString());
    const check = setInterval(() => {
      const elapsed = new Date() - pivotTime;
      if (elapsed > 3000) {
        pivotTime = new Date();
        console.log('Still waiting for satisfaction of ' +
            predicate.toString());
      } else if (predicate()) {
        clearInterval(check);
        resolve();
      }
    }, 50);
  });
}

/**
 * @param {string} reason
 */
function failTest(reason) {
  result = 'FAIL: ' + reason;
  console.log('Test Failed: ', reason);
  testDone = true;
  // Cause test termination.
  throw new Error(reason);
}

function reportTestSuccess() {
  result = 'PASS';
  console.log('Test Passed');
  testDone = true;
}

/**
 * @param {string} expected
 * @param {string} actual
 */
function assertEquals(expected, actual) {
  if (actual !== expected) {
    failTest('expected "' + expected + '", got "' + actual + '".');
  }
}

/**
 * @param {boolean} booleanExpression
 * @param {string} reason
 */
function assertTrue(booleanExpression, reason) {
  if (!booleanExpression) {
    failTest(reason);
  }
}

/**
 * @param {MediaStream} stream
 * @param {string} mimeType
 * @param {number} slice The number of milliseconds to record into each blob
 * @return {Promise<MediaRecorder>}
 */
function createAndStartMediaRecorder(stream, mimeType, slice) {
  return new Promise((resolve, reject) => {
    document.getElementById('video').srcObject = stream;
    const recorder = new MediaRecorder(stream, {'mimeType': mimeType});
    console.log('Recorder object created.');
    if (slice != undefined) {
      recorder.start(slice);
      console.log('Recorder started with time slice', slice);
    } else {
      recorder.start(1);
    }
    resolve(recorder);
  });
}

/**
 * @param {MediaStream} stream
 * @param {string} mimeType
 * @return {Promise<MediaRecorder>}
 */
function createMediaRecorder(stream, mimeType) {
  return new Promise((resolve, reject) => {
    const recorder = new MediaRecorder(stream, {'mimeType': mimeType});
    console.log('Recorder object created.');
    resolve(recorder);
  });
}

/**
 * Tests that the MediaRecorder's start() function will cause the |state| to be
 * 'recording' and that a 'start' event is fired.
 */
function testStartAndRecorderState() {
  let startEventReceived = false;
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        recorder.onstart = (event) => {
          startEventReceived = true;
          assertEquals('recording', recorder.state);
        };
        recorder.start(1);
      })
      .then(() => {
        return waitFor('Make sure the start event was received',
            () => {
              return startEventReceived;
            });
      })
      .catch((err) => {
        return failTest(err.toString());
      })
      .then(() => {
        reportTestSuccess();
      });
}

/**
 * Tests that the MediaRecorder's stop() function will effectively cause the
 * |state| to be 'inactive' and that a 'stop' event is fired.
 */
function testStartStopAndRecorderState() {
  let stopEventReceived = false;
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createAndStartMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        recorder.onstop = (event) => {
          stopEventReceived = true;
          assertEquals('inactive', recorder.state);
        };
        recorder.stop();
      })
      .then(() => {
        return waitFor('Make sure the stop event was received',
            () => {
              return stopEventReceived;
            });
      })
      .catch((err) => {
        return failTest(err.toString());
      })
      .then(() => {
        reportTestSuccess();
      });
}

/**
 * Tests that when MediaRecorder's start() function is called, some data is
 * made available by media recorder via dataavailable events, containing non
 * empty blob data.
 * @param {string} type The MIME type string
 */
function testStartAndDataAvailable(type) {
  let videoSize = 0;
  let emptyBlobs = 0;
  const timeStamps = [];

  if (!MediaRecorder.isTypeSupported(type)) {
    // If the MIME type is not supported, there's nothing needed to test.
    // Mark as success.
    reportTestSuccess();
    return;
  }

  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createAndStartMediaRecorder(stream, type);
      })
      .then((recorder) => {
        // Save history of Blobs received via dataavailable.
        recorder.ondataavailable = (event) => {
          timeStamps.push(event.timeStamp);
          if (event.data.size > 0) {
            videoSize += event.data.size;
          } else {
            emptyBlobs += 1;
          }
        };
      })
      .then(() => {
        return waitFor('Make sure the recording has data',
            () => {
              return videoSize > 0;
            });
      })
      .then(() => {
        assertTrue(emptyBlobs == 0, 'Recording has ' + emptyBlobs +
            ' empty blobs, there should be no such empty blobs.');
      })
      .catch((err) => {
        return failTest(err.toString() + ' MIME type: ' + type);
      })
      .then(() => {
        reportTestSuccess();
      });
}

window.onload = () => {
  pageLoaded = true;
};

