/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

window.addEventListener('load', () => {
  const dropArea = document.getElementById('drag-drop');
  dropArea.ondragover = function (e) { e.preventDefault(); }
  dropArea.ondragenter = function (e) { e.preventDefault(); }

  dropArea.ondrop = function (e) {
    e.preventDefault();
    if (e.dataTransfer.files.length === 1) {
      window.document.title = 'drop registered:' + e.dataTransfer.files[0].name;
      dropArea.innerText = 'drop registered';
    }
  }

  window.document.title = 'awaiting drop.';
}, { once: true });

