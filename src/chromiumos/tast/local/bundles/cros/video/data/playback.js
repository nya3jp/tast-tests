// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

const videoClass = 'videoTile';

function setGridSize(dimension) {
  if (dimension <= 0) return;

  // Find the |container| and make it a |dimension| x |dimension| grid; repeat()
  // allows for automatically ordering sub-grids into |dimension| columns, see
  // https://developer.mozilla.org/en-US/docs/Web/CSS/grid-template-columns
  const container = document.getElementById('container');
  container.style.display = 'grid';
  container.style.gridTemplateColumns = 'repeat(' + dimension + ', 1fr)';

  // Adjust until we get |dimension| x |dimension| videos.
  const numVideos = document.getElementsByClassName(videoClass).length;
  const numExtraVideosInGrid = dimension * dimension - numVideos;
  if (numExtraVideosInGrid > 0) {
    for (let i = 0; i < numExtraVideosInGrid; i++) {
      const video = document.createElement('video');
      video.className = videoClass;
      video.controls = true;
      video.autoplay = true;
      video.muted = true;
      const div = document.createElement('div');
      div.appendChild(video);
      container.appendChild(div);
    }
  } else {
    for (let i = 0; i > numExtraVideosInGrid; i--) {
      container.removeChild(container.lastChild);
    }
  }
}

async function playUntilEnd(videoSourcePath, unmutePlayer) {
  let videos = Array.from(document.getElementsByClassName(videoClass));
  videos.forEach(async video => {
    video.src = videoSourcePath;
    video.muted = !unmutePlayer;
    await video.play();
  }
  );
  console.log('Loaded url: ', videoSourcePath);
}

async function playRepeatedly(videoSourcePath) {
  let videos = Array.from(document.getElementsByClassName(videoClass));
  videos.forEach(async video => {
    video.src = videoSourcePath;
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
