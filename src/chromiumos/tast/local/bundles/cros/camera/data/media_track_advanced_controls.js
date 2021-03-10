// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

var cameraDeviceIds = [];
var cameraDeviceLabels = [];

function getPreviewVideo() {
  return document.querySelector('#preview-video');
}

/**
 * Gets number of cameras.
 * @return {number}
 */
async function getNumOfCameras() {
  const devices = await navigator.mediaDevices.enumerateDevices();
  for (var i = 0; i < devices.length; i++)
  {
    var tempDevice = devices[i];
    if (tempDevice.kind == "videoinput")
    {
      cameraDeviceIds.push(tempDevice.deviceId);
      cameraDeviceLabels.push(tempDevice.label);
    }
  }

  return cameraDeviceIds.length;
}

function getCameraLabel(index) {
  return cameraDeviceLabels[index];
}

async function playVideo(index) {
  video = getPreviewVideo();
  video.autoplay = true;
  console.log("use device id" + cameraDeviceIds[index])
  constraints = {
    'audio': false,
    'video': {
        'deviceId': cameraDeviceIds[index],
    },
  };
  await navigator.mediaDevices.getUserMedia(constraints)
    .then(function (stream) {
      video.srcObject = stream;
    })
    .catch(function (error) {
      console.log(error);
    });
}

function stopVideo() {
  video = getPreviewVideo();
  var stream = video.srcObject;
  if (stream) {
    var tracks = stream.getTracks();

    for (var i = 0; i < tracks.length; i++) {
      var track = tracks[i];
      track.stop();
    }
  }

  video.srcObject = null;
}
/**
 * Gets media track capabilities of current active camera device.
 * @return {!Promise<!MediaTrackCapabilities>}
 */
async function getMediaTrackCapabilities() {
  video = getPreviewVideo();
  const track = video.srcObject.getVideoTracks()[0];
  const capabilities = await track.getCapabilities();
  return capabilities;
}

/**
 * Gets media track settings of current active camera device.
 * @return  {!Promise<!MediaTrackSettings>}
 */
async function getMediaTrackSettings() {
  const track = getPreviewVideo().srcObject.getVideoTracks()[0];
  const settings = await track.getSettings();
  return settings;
}

/**
 * Apply media track constraints of current active camera device.
 * @param {string} constraints
 * @return  {!Promise}
 */
async function applyMediaTrackConstraints(constraints) {
  const track = getPreviewVideo().srcObject.getVideoTracks()[0];
  console.log(constraints)
  await track.applyConstraints(JSON.parse(constraints));
}
