// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

async function playUntilEnd(videoSourcePath, unmutePlayer) {
  let videos = document.getElementsByName('media');
  videos.forEach(async video => {
    video.src = videoSourcePath;
    video.muted = !unmutePlayer;
    await video.play();
  }
  );
  console.log('Loaded url: ', videoSourcePath);
}

async function playRepeatedly(videoSourcePath) {
  let videos = document.getElementsByName("media");
  videos.forEach(async video => {
    video.src = videoSourcePath;
    video.muted = true;
    video.loop = true;
    await video.play();
  }
  );
  console.log('Loaded url: ', videoSourcePath);
}

// Quick and dirty randomizer that returns the same sequence of numbers provided
// the same seed is used.
function randomizer(seed) {
  return function () {
    const x = Math.sin(seed++) * 10000;
    return x - Math.floor(x);
  }
};

const random = randomizer(1);
let number_finished_seeks = 0;
function randomSeek() {
  return new Promise((resolve, reject) => {
    video.onseeked = (event) => {
      console.log(number_finished_seeks);
      resolve(number_finished_seeks++);
    };
    video.onerror = (event) => {
      // |event| is a MediaError which has |code| and |message|.
      reject(new Error('onerror event message: ' + event.message +
        ', video.error.message: ' + video.error.message));
    };
    video.currentTime = random() * video.duration;
  });
}

window.addEventListener("keydown", function (event) {
  if (event.key == 'f')
    document.getElementById('video').requestFullscreen();
});
