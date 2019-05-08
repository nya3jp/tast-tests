// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
 * Asserts that the given function should throw an exception.
 * @param {function()} func
 * @param {string} description to explain the function
 */
function assertThrows(func, description) {
  try {
    func.call();
    failTest('Error:' + func + description + ' did not throw!');
  } catch (e) {
    console.log(e);
    reportTestSuccess();
  }
}

/**
 * @param {MediaStream} stream
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

/**
 * Tests that when MediaRecorder's start(timeSlice) is called, some data
 * available events are fired containing non empty blob data.
 */
function testStartWithTimeSlice() {
  let videoSize = 0;
  let emptyBlobs = 0;
  const defaultTimeSlice = 100;
  const timeStamps = [];
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createAndStartMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE,
            defaultTimeSlice);
      })
      .then((recorder) => {
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
        return waitFor('Making sure the recording has data',
            () => {
              return videoSize > 0 && timeStamps.length > 10;
            });
      })
      .then(() => {
        assertTrue(emptyBlobs == 0, 'Recording has ' + emptyBlobs +
            ' empty blobs, there should be no such empty blobs.');
      })
      .catch((err) => {
        return failTest(err.toString());
      })
      .then(() => {
        reportTestSuccess();
      });
}

/**
 * Tests that when a MediaRecorder's resume() is called, the |state| is
 * 'recording' and a 'resume' event is fired.
 */
function testResumeAndRecorderState() {
  let theRecorder;
  let resumeEventReceived = false;
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createAndStartMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        theRecorder = recorder;
        theRecorder.pause();
      })
      .then(() => {
        theRecorder.onresume = (event) => {
          resumeEventReceived = true;
          assertEquals('recording', theRecorder.state);
        };
        theRecorder.resume();
      })
      .then(() => {
        return waitFor('Making sure the resume event has been received',
            () => {
              return resumeEventReceived;
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
 * Tests that it is not possible to resume an inactive MediaRecorder.
 */
function testIllegalResumeThrowsDOMError() {
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        assertThrows(() => {
          recorder.resume();
        },
        'Calling resume() in' +
            ' inactive state should cause a DOM error');
      });
}

/**
 * Tests that MediaRecorder sends data blobs when resume() is called.
 */
function testResumeAndDataAvailable() {
  let videoSize = 0;
  let emptyBlobs = 0;
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createAndStartMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        recorder.pause();
        recorder.ondataavailable = (event) => {
          if (event.data.size > 0) {
            videoSize += event.data.size;
          } else {
            console.log('This dataavailable event is empty', event);
            emptyBlobs += 1;
          }
        };
        recorder.resume();
      })
      .then(() => {
        return waitFor('Make sure the recording has data after resuming',
            () => {
              return videoSize > 0;
            });
      })
      .then(() => {
        // There should be no empty blob while recording.
        assertTrue(emptyBlobs == 0, 'Recording has ' + emptyBlobs +
            ' empty blobs, there should be no such empty blobs.');
      })
      .catch((err) => {
        return failTest(err.toString());
      })
      .then(() => {
        reportTestSuccess();
      });
}

/**
 * Tests that when paused the recorder will transition |state| to |paused| and
 * then trigger a |pause| event.
 */
function testPauseAndRecorderState() {
  let pauseEventReceived = false;
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createAndStartMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        recorder.onpause = (event) => {
          pauseEventReceived = true;
          assertEquals('paused', recorder.state);
        };
        recorder.pause();
      })
      .then(() => {
        return waitFor('Making sure the pause event has been received',
            () => {
              return pauseEventReceived;
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
 * Tests that it is possible to stop a paused MediaRecorder and that the |state|
 * becomes 'inactive'.
 */
function testPauseStopAndRecorderState() {
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createAndStartMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        recorder.pause();
        recorder.stop();
        assertEquals('inactive', recorder.state);
      })
      .catch((err) => {
        return failTest(err.toString());
      })
      .then(() => {
        reportTestSuccess();
      });
}

/**
 * Tests that no dataavailable event is fired after MediaRecorder's pause()
 * function is called.
 */
function testPausePreventsDataavailableFromBeingFired() {
  const waitDuration = (duration) => {
    return new Promise((resolve, reject) => {
      console.log('Waiting for ', duration.toString(), 'msec');
      setTimeout(
          () => {
            console.log('Done waiting');
            resolve();
          }, duration);
    });
  };
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createAndStartMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        recorder.pause();
        recorder.ondataavailable = (event) => {
          failTest('Received unexpected data after pause!');
        };
      })
      .then(() => {
        // Waits for data gathering.
        return waitDuration(2000);
      })
      .catch((err) => {
        return failTest(err.toString());
      })
      .then(() => {
        reportTestSuccess();
      });
}

/**
 * Tests that MediaRecorder's pause() throws an exception if |state| is not
 * 'recording'.
 */
function testIllegalPauseThrowsDOMError() {
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        assertThrows(() => {
          recorder.pause();
        },
        'Calling pause() in' +
            ' inactive state should cause a DOM error');
      });
}

/**
 * Tests that MediaRecorder's stop() throws an exception if |state| is not
 * 'recording'.
 */
function testIllegalStopThrowsDOMError() {
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        assertThrows(() => {
          recorder.stop();
        },
        'Calling stop() in' +
            ' inactive state should cause a DOM error');
      });
}

/**
 * Tests that MediaRecorder's start() throws an exception if |state| is
 * 'recording'.
 */
function testIllegalStartInRecordingStateThrowsDOMError() {
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createAndStartMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        assertThrows(() => {
          recorder.start(1);
        },
        'Calling start() in' +
            ' recording state should cause a DOM error');
      });
}

/**
 * Tests that MediaRecorder's start() throws an exception if |state| is
 * 'paused'.
 */
function testIllegalStartInPausedStateThrowsDOMError() {
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createAndStartMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        recorder.pause();
        assertThrows(() => {
          recorder.start(1);
        },
        'Calling start(1) in' +
            ' paused state should cause a DOM error');
      });
}

/**
 * Tests that MediaRecorder can record a 2 Channel audio stream.
 */
function testTwoChannelAudio() {
  let audioSize = 0;
  const defaultFrequency = 880;
  const context = new AudioContext();
  const oscillator = context.createOscillator();
  oscillator.type = 'sine';
  oscillator.frequency.value = defaultFrequency;
  const dest = context.createMediaStreamDestination();
  dest.channelCount = 2;
  oscillator.connect(dest);
  createMediaRecorder(dest.stream, DEFAULT_RECORDER_MIME_TYPE)
      .then((recorder) => {
        recorder.ondataavailable = (event) => {
          audioSize += event.data.size;
        };
        recorder.start(1);
        oscillator.start();
      })
      .then(() => {
        return waitFor('Make sure the recording has data',
            () => {
              return audioSize > 0;
            });
      })
      .catch((err) => {
        return failTest(err.toString());
      })
      .then(() => {
        console.log('audioSize', audioSize);
        reportTestSuccess();
      });
}

/**
 * Tests that MediaRecorder's requestData() throws an exception if |state| is
 * 'inactive'.
 */
function testIllegalRequestDataThrowsDOMError() {
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        return createMediaRecorder(stream, DEFAULT_RECORDER_MIME_TYPE);
      })
      .then((recorder) => {
        assertThrows(() => {
          recorder.requestData();
        },
        'Calling requestdata() in inactive state should throw a DOM ' +
            'Exception');
      });
}

/**
 * Tests that MediaRecorder fires an Error Event when the associated MediaStream
 * gets a Track added.
 */
function testAddingTrackToMediaStreamFiresErrorEvent() {
  let theStream;
  let errorEventReceived = false;
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        theStream = stream;
        return createMediaRecorder(stream);
      })
      .then((recorder) => {
        recorder.onerror = (event) => {
          errorEventReceived = true;
        };
        recorder.start(1);
        // Add a new track, copy of an existing one for simplicity.
        theStream.addTrack(theStream.getTracks()[1].clone());
      })
      .then(() => {
        return waitFor('Waiting for the Error Event',
            () => {
              return errorEventReceived;
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
 * Tests that MediaRecorder fires an Error Event when the associated MediaStream
 * gets a Track removed.
 */
function testRemovingTrackFromMediaStreamFiresErrorEvent() {
  let theStream;
  let errorEventReceived = false;
  navigator.mediaDevices.getUserMedia(DEFAULT_CONSTRAINTS)
      .then((stream) => {
        theStream = stream;
        return createMediaRecorder(stream);
      })
      .then((recorder) => {
        recorder.onerror = (event) => {
          errorEventReceived = true;
        };
        recorder.start(1);
        theStream.removeTrack(theStream.getTracks()[1]);
      })
      .then(() => {
        return waitFor('Waiting for the Error Event',
            () => {
              return errorEventReceived;
            });
      })
      .catch((err) => {
        return failTest(err.toString());
      })
      .then(() => {
        reportTestSuccess();
      });
}

window.onload = () => {
  pageLoaded = true;
};

