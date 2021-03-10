// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/**
 * @type {!Array<!MediaDeviceInfo>}
 */
let allDeviceInfo = [];

async function initTest() {
  allDeviceInfo = (await navigator.mediaDevices.enumerateDevices())
                        .filter((device) => device.kind === 'videoinput');
}

/**
 * @return {!HTMLVideoElement}
 */
function getPreviewVideo() {
  return document.querySelector('#preview-video');
}

/**
 * Gets number of cameras.
 * @return number
 */
function getNumOfCameras() {
  return allDeviceInfo.length;
}

function getCameraLabel(index) {
  return allDeviceInfo[index].label;
}

/**
 * Open preview from specified camera.
 *
 * @param {number}
 * @throws {!Error}
 */
async function openPreview(index) {
  const video = getPreviewVideo();
  const constraints = {
    'audio': false,
    'video': {
        'deviceId': allDeviceInfo[index].deviceId,
    },
  };
  try {
    const stream = await navigator.mediaDevices.getUserMedia(constraints);
    video.srcObject = stream;
    await video.play();
  } catch (error) {
    throw error; // re-throw error to make test failed here.
  }
}

/**
 * Close preview.
 */
function closePreview() {
  const video = getPreviewVideo();
  var stream = video.srcObject;
  if (stream === undefined) {
    throw new Error('Stream is undefined'); // throw error to make test fail.
  }
  for (const track of stream.getTracks()) {
    track.stop();
  }

  video.removeAttribute('srcObject');
  video.load();
}

/**
 * Gets media track capabilities of current active camera device.
 * @return {!Promise<!MediaTrackCapabilities>}
 */
async function getMediaTrackCapabilities() {
  const video = getPreviewVideo();
  if (video.srcObject === undefined) {
   throw new Error('video.srcObject is undefined');
  }
  const track = video.srcObject.getVideoTracks()[0];
  const capabilities = await track.getCapabilities();
  return capabilities;
}

/**
 * Gets media track settings of current active camera device.
 * @return  {!Promise<!MediaTrackSettings>}
 */
async function getMediaTrackSettings() {
  const video = getPreviewVideo();
  if (video.srcObject === undefined) {
   throw new Error('video.srcObject is undefined');
  }
  const track = video.srcObject.getVideoTracks()[0];
  const settings = await track.getSettings();
  return settings;
}

/**
 * Apply media track constraints of current active camera device.
 * @param {string} constraints
 * @return  {!Promise}
 */
async function applyMediaTrackConstraints(constraints) {
  const video = getPreviewVideo();
  if (video.srcObject === undefined) {
   throw new Error('video.srcObject is undefined');
  }
  const track = video.srcObject.getVideoTracks()[0];
  await track.applyConstraints(JSON.parse(constraints));
}
