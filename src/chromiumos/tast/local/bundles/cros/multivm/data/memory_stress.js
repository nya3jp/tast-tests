// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

function alloc(sizeMB, randomRatio, retouchRatio) {
  const FLOAT64_BYTES = 8;
  const MB = 1024 * 1024;
  const FLOAT64_PER_PAGE = 4096 / FLOAT64_BYTES;
  const totalCount = sizeMB * MB / FLOAT64_BYTES;
  const randomCount = totalCount * randomRatio;
  // Using Float64Array as each element of Float64Array should consume 64
  // bits memory. Each elements of Uint32Array might consume 64 bits memory
  // on 64-bit architecture.
  // Random array is uncompressible.
  const randPerPage = FLOAT64_PER_PAGE * randomRatio;
  const buffer = new Float64Array(totalCount);
  function randomFill(bytes) {
    for (let i = 0; i < bytes; i += FLOAT64_PER_PAGE) {
      for (let j = i; j < i + randPerPage; j++) {
        buffer[j] = Math.random();
      }
    }
  }
  randomFill(buffer.length);

  const retouchBytes = Math.floor(sizeMB * retouchRatio) * MB;
  if (retouchBytes > 0) {
    let timesTouched = 0;
    const touchDiv = document.getElementById('touch');
    function periodicTouch() {
      timesTouched++;
      randomFill(retouchBytes);
      touchDiv.textContent = `touched ${timesTouched} times`;
      setTimeout(periodicTouch, 1000);
    }
    setTimeout(periodicTouch, 1000);
  }

  return buffer;
}

function main() {
  const url = new URL(window.location.href);
  const allocMb = parseInt(url.searchParams.get('alloc'));
  const randomRatio = parseFloat(url.searchParams.get('random'));
  const retouchRatio = parseFloat(url.searchParams.get('retouch'));

  const startTime = new Date();
  // Assigns the content to document to avoid optimization of unused data.
  document.out = alloc(allocMb, randomRatio, retouchRatio);
  const ellapse = (new Date() - startTime) / 1000;
  // Shows the loading time for manual test.
  const content = `Allocating ${allocMb} MB takes ${ellapse} seconds`;
  document.getElementById('display').textContent = content;
}

// Allocates javascript objects after the first rendering.
window.addEventListener('DOMContentLoaded', () => { setTimeout(main); });
