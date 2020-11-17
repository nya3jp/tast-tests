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
  document.addEventListener('paste', (e) => {
    e.preventDefault();
    if (e.clipboardData.types.includes('fs/sources')) {
      const itemTitle = e.clipboardData.getData('fs/sources').split('/').pop();
      window.document.title = 'paste registered:' + itemTitle;
      window.myData = {};
      window.myData["files"] = [];
      window.myData["fileCount"] = e.clipboardData.files.length;
      for (let i = 0; i < e.clipboardData.files.length; i++) {
        let file = e.clipboardData.files.item(i);
        window.myData["files"].push(file);
      }
      e.clipboardData.types.forEach((type) => {
        window.myData[type] = e.clipboardData.getData(type);
      })
      dropArea.innerHTML = 'paste registered';
    } else {
      console.error("paste event doesn't contain fs/sources")
    }
  });

}, { once: true });

