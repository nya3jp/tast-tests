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
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: false,
      video: {
        facingMode: {
          ideal: 'user',
        },
      },
    });

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
