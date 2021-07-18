// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.


  // var result = null;

  var contentArea = document.getElementById('drag-drop');
var scanButton = document.getElementById('scanButton');

scanButton.addEventListener('click', function() {
  document.addEventListener('copy', (event) => {
    event.clipboardData.setData("text/plain", document.getSelection());
    event.preventDefault();
    contentArea.innerText = document.getSelection();
    window.document.title = 'content copied.';
  }, {once: true});
  if (!document.execCommand('copy')) {
    throw new Error('Failed to execute copy');
  }
});
