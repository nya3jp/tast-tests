// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

let localPeerConnection = new RTCPeerConnection();
let remotePeerConnection = new RTCPeerConnection();

async function start(
    profile, isSimulcast, svcScalabilityMode = '', width = 1280, height = 720) {
  const constraints = {audio : false, video : {width : width, height : height}};

  localPeerConnection.onicecandidate = e =>
      remotePeerConnection.addIceCandidate(e.candidate);
  remotePeerConnection.onicecandidate = e =>
      localPeerConnection.addIceCandidate(e.candidate);
  const onTrack = new Promise((resolve, reject) => {
    remotePeerConnection.ontrack = e => {
      const remoteVideo = document.getElementById('remoteVideo');
      remoteVideo.srcObject = e.streams[0];
      resolve();
    };
  });

  // |targetBitrate| uses a conservative 0.05 bits per pixel (bpp) estimate.
  const targetBitrate = width * height * 30 /*fps*/ * 0.05;

  if (isSimulcast) {
    await runLoopbackPeerConnectionWithSimulcast(constraints, targetBitrate);
  } else if (svcScalabilityMode !== '') {
    await runLoopbackPeerConnectionWithSVC(
        constraints, svcScalabilityMode, profile, targetBitrate);
  } else {
    await runLoopbackPeerConnection(constraints, profile, targetBitrate);
  }
  await onTrack;
}

async function runLoopbackPeerConnection(constraints, profile, targetBitrate) {
  const stream = await navigator.mediaDevices.getUserMedia(constraints);
  localPeerConnection.addTransceiver(stream.getVideoTracks()[0], {
    // Prefer resolution even at the cost of visual quality to avoid falling
    // down to SW video encoding, see b/181320567 or crbug.com/1179020.
    degradationPreference: 'maintain-resolution',
    streams : [ stream ],
  });

  const offer = await localPeerConnection.createOffer();
  if (profile) {
    offer.sdp = setSdpDefaultVideoCodec(offer.sdp, profile, false, '');
  }
  await localPeerConnection.setLocalDescription(offer);
  await remotePeerConnection.setRemoteDescription(
      localPeerConnection.localDescription);

  const answer = await remotePeerConnection.createAnswer();
  answer.sdp = appendStartBitrateToSDP(answer.sdp, profile, targetBitrate);
  await remotePeerConnection.setLocalDescription(answer);
  await localPeerConnection.setRemoteDescription(
      remotePeerConnection.localDescription);
}

async function runLoopbackPeerConnectionWithSimulcast(constraints,
                                                      targetBitrate) {
  const rids = [ 0, 1, 2 ];
  const stream = await navigator.mediaDevices.getUserMedia(constraints);
  localPeerConnection.addTransceiver(stream.getVideoTracks()[0], {
    // Prefer resolution even at the cost of visual quality to avoid falling
    // down to SW video encoding, see b/181320567 or crbug.com/1179020.
    degradationPreference: 'maintain-resolution',
    streams : [ stream ],
    sendEncodings: rids.map(rid => {
      return {'rid': rid, 'scaleResolutionDownBy': (2 ** rid)};
    }),
  });

  const offer = await localPeerConnection.createOffer();
  await localPeerConnection.setLocalDescription(offer);
  await remotePeerConnection.setRemoteDescription({
    type : 'offer',
    sdp : swapRidAndMidExtensionsInSimulcastOffer(offer, rids),
  });

  const answer = await remotePeerConnection.createAnswer();
  answer.sdp = appendStartBitrateToSDP(answer.sdp, 'VP8', targetBitrate);
  await remotePeerConnection.setLocalDescription(answer);
  await localPeerConnection.setRemoteDescription({
    type : 'answer',
    sdp : swapRidAndMidExtensionsInSimulcastAnswer(
        answer, localPeerConnection.localDescription, rids),
  });
}

async function runLoopbackPeerConnectionWithSVC(
    constraints, svcScalabilityMode, profile, targetBitrate) {
  const stream = await navigator.mediaDevices.getUserMedia(constraints);
  localPeerConnection.addTransceiver(stream.getVideoTracks()[0], {
    // Prefer resolution even at the cost of visual quality to avoid falling
    // down to SW video encoding, see b/181320567 or crbug.com/1179020.
    degradationPreference: 'maintain-resolution',
    streams : [ stream ],
    sendEncodings : [{"scalabilityMode": svcScalabilityMode}]
  });

  const offer = await localPeerConnection.createOffer();
  if (profile) {
    offer.sdp = setSdpDefaultVideoCodec(offer.sdp, profile, false, '');
  }
  await localPeerConnection.setLocalDescription(offer);
  await remotePeerConnection.setRemoteDescription(
      localPeerConnection.localDescription);

  const answer = await remotePeerConnection.createAnswer();
  answer.sdp = appendStartBitrateToSDP(answer.sdp, profile, targetBitrate);
  await remotePeerConnection.setLocalDescription(answer);
  await localPeerConnection.setRemoteDescription(
      remotePeerConnection.localDescription);
}

// Returns true if the video frame being displayed is considered "black".
// Specifying |width| or |height| smaller than the feeding |remoteVideo| can be
// used for speeding up the calculation by downscaling.
function isBlackVideoFrame(width = 1280, height = 720) {
  const context = new OffscreenCanvas(width, height).getContext('2d');

  const remoteVideo = document.getElementById('remoteVideo');
  context.drawImage(remoteVideo, 0, 0, width, height);
  const imageData = context.getImageData(0, 0, width, height);
  return isBlackFrame(imageData.data, imageData.data.length);
}

const IDENTICAL_FRAME_SSIM_THRESHOLD = 0.99;
// Returns true if the previous video frame is too similar to the current video
// frame, implying that the video feed is frozen. The similarity is calculated
// using ssim() and comparing with the IDENTICAL_FRAME_SSIM_THRESHOLD.
// Specifying |width| or |height| smaller than the feeding |remoteVideo| can be
// used for speeding up the calculation by downscaling.
function isFrozenVideoFrame(width = 1280, height = 720) {
  const context = new OffscreenCanvas(width, height).getContext('2d');

  const remoteVideo = document.getElementById('remoteVideo');
  context.drawImage(remoteVideo, 0, 0, width, height);
  const imageData = context.getImageData(0, 0, width, height);

  if (isFrozenVideoFrame.previousImageData == null) {
    isFrozenVideoFrame.previousImageData = imageData;
    return false;
  }

  const ssim = new Ssim();
  const ssimValue =
      ssim.calculate(imageData.data, isFrozenVideoFrame.previousImageData.data)
  isFrozenVideoFrame.previousImageData = imageData;
  return ssimValue > IDENTICAL_FRAME_SSIM_THRESHOLD;
}

// Transforms the "container" <div> that holds the real time <video> into a
// |dimension| x |dimension| grid, and fills it with |videoURL| <video>s.
// Reusing the same URL being played back should not affect the test since each
// <video> will need to decode and play independently from the others.
function makeVideoGrid(dimension, videoURL) {
  // Find the |container| and make it a |dimension| x |dimension| grid; repeat()
  // allows for automatically ordering sub-grids into |dimension| columns, see
  // https://developer.mozilla.org/en-US/docs/Web/CSS/grid-template-columns
  const container = document.getElementById('container');
  container.style.display = 'grid';
  container.style.gridTemplateColumns = 'repeat(' + dimension + ', 1fr)';

  // Fill the grid with <video>s. Note that there is already one <video> in the
  // grid for the remote RTCPeerConnection stream feed.
  const numExtraVideosInGrid = dimension * dimension - 1;
  for (let i = 0; i < numExtraVideosInGrid; i++) {
    const video = document.createElement('video');
    video.src = videoURL;
    video.style.maxWidth = '100%';
    video.autoplay = true;
    video.muted = true;
    video.loop = true;
    const div = document.createElement('div');
    div.appendChild(video);
    container.appendChild(div);
  }
}

// Appends a 'a=fmtp:bla x-google-start-bitrate=foo' statement to |sdp|, where
// 'bla' is the SDP id for |profile| and 'foo' is the |startBitrate| in Kbps;
// this statement is needed to prevent RTCPeerConnections from dropping
// resolution to keep a by-default low start bitrate.
function appendStartBitrateToSDP(sdp, profile, startBitrate) {
  const codec_id = findRtpmapId(splitSdpLines(sdp), profile);
  if (codec_id) {
    const targetBitrateKbpsAsInt = Math.trunc(startBitrate / 1000);
    sdp += `a=fmtp:${codec_id} ` +
        `x-google-start-bitrate=${targetBitrateKbpsAsInt}\r\n`;
  }
  return sdp;
}
