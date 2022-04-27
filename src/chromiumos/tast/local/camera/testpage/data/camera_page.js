// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/**
 * @typedef {{
 *   hasEnded: boolean,
 * }}
 */
let TrackState;

// Namespace for calling from Tast.
window.Tast = {
  /**
   * Starts a camera stream and returns a TrackState object to track the
   * stream status.
   * @return {!Promise<!TrackState>}
   */
  async startStream() {
    return Tast.startStreamWithConstraints({
      audio: false,
      video: true,
    });
  },

  /**
   * Starts a camera stream and returns a TrackState object to track the
   * stream status.
   * @param {Object} constraints
   * @return {!Promise<!TrackState>}
   */
  async startStreamWithConstraints(constraints) {
    console.log(
        "getUserMedia with constraints: " + JSON.stringify(constraints));
    const stream = await navigator.mediaDevices.getUserMedia(constraints);
    const video = document.querySelector('video');
    video.srcObject = stream;

    const track = stream.getVideoTracks()[0];
    const trackState = {hasEnded: false};
    track.addEventListener('ended', () => {
      trackState.hasEnded = true;
    });
    return trackState;
  }
};
