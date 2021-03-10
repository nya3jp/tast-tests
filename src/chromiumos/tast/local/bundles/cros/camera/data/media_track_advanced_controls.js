// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/**
 * @type {!Array<!MediaDeviceInfo>}
 */
let allDeviceInfo = [];

/**
 * Get camera device info.
 * @return !Promise<!Array<!MediaDeviceInfo>>
 */
async function getCameras() {
  var cameraDeviceInfo = [];
  allDeviceInfo = (await navigator.mediaDevices.enumerateDevices())
                        .filter((device) => device.kind === 'videoinput');
  for (var i = 0; i < allDeviceInfo.length; i++)
    cameraDeviceInfo.push({'deviceId': allDeviceInfo[i].deviceId,
        'label': allDeviceInfo[i].label});
  return cameraDeviceInfo;
}

/**
 * @return {!HTMLVideoElement}
 */
function getPreviewVideo() {
  return document.querySelector('#preview-video');
}

/**
 * @return {!HTMLVideoElement}
 */
function getPreviewVideoWithStream() {
  const video = getPreviewVideo();
  if (video.srcObject === undefined) {
    throw new Error('Stream is undefined');
  }
  return video;
}

/**
 * Opens preview from specified camera.
 *
 * @param {deviceId}
 * @throws {!Error}
 */
async function openPreview(deviceId) {
  var video = getPreviewVideo();
  const constraints = {
    'audio': false,
    'video': {deviceId},
  };
  try {
    const stream = await navigator.mediaDevices.getUserMedia(constraints);
    video.srcObject = stream;
    await video.play();
  } catch (error) {
    throw error;
  }
}

/**
 * Closes preview.
 */
function closePreview() {
  const video = getPreviewVideoWithStream();
  const stream = video.srcObject;
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
  const video = getPreviewVideoWithStream();
  const track = video.srcObject.getVideoTracks()[0];
  const capabilities = await track.getCapabilities();
  return capabilities;
}

/**
 * Gets media track settings of current active camera device.
 * @return {!MediaTrackSettings}
 */
function getMediaTrackSettings() {
  const video = getPreviewVideoWithStream();
  const track = video.srcObject.getVideoTracks()[0];
  const settings = track.getSettings();
  return settings;
}

/**
 * Applies media track constraints of current active camera device.
 * @param {string} constraints
 * @return  {!Promise}
 */
async function applyMediaTrackConstraints(constraints) {
  const video = getPreviewVideoWithStream();
  const track = video.srcObject.getVideoTracks()[0];
  await track.applyConstraints(constraints);
}
