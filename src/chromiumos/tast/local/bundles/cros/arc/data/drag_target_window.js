/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

window.addEventListener('load', () => {
  let dropArea = document.getElementById('drop-area');
  dropArea.addEventListener('dragover', (event) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "copy";
  });
  dropArea.addEventListener('drop', (event) => {
    event.preventDefault();
    document.getElementById('dropped-data').innerHTML =
        event.dataTransfer.getData('text/plain');
  });
}, {once: true});
