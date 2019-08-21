// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

function alloc(sizeMB, randomRatio) {
  const FLOAT64_BYTES = 8;
  const MB = 1024 * 1024;
  const totalCount = sizeMB * MB / FLOAT64_BYTES;
  const randomCount = totalCount * randomRatio;
  // Using Float64Array as each element of Float64Array should consume 64
  // bits memory. Each elements of Uint32Array might consume 64 bits memory
  // on 64-bit architecture.
  // Random array is uncompressable.
  const randomArray = new Float64Array(randomCount);
  for (let i = 0; i < randomArray.length; i++) {
    randomArray[i] = Math.random();
  }
  // Constant array is compressable.
  const constCount = totalCount * (1 - randomRatio);
  const constArray = new Float64Array(constCount);
  for (let i = 0; i < constArray.length; i++) {
    constArray[i] = 1;
  }
  return [randomArray, constArray];
}

function main() {
  const url = new URL(window.location.href);
  const allocMb = parseInt(url.searchParams.get('alloc'));
  const randomRatio = parseFloat(url.searchParams.get('ratio'));

  const startTime = new Date();
  // Assigns the content to docuement to avoid optimization of unused data.
  document.out = alloc(allocMb, randomRatio);
  const ellapse = (new Date() - startTime) / 1000;
  // Shows the loading time for manual test.
  const content = `Allocating ${allocMb} MB takes ${ellapse} seconds`
  document.getElementById('display').textContent = content;
}

// Allocates javascript objects after the first rendering.
setTimeout(main);
