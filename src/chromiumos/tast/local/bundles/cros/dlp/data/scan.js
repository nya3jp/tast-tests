// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.


  // var result = null;

  var contentArea = document.getElementById('drag-drop');
var scanButton = document.getElementById('scanButton');

scanButton.addEventListener('click', function() {
  let result = contentArea.innerText;

  document.addEventListener('paste', (event) => {
    let content = event.clipboardData.getData('text/plain').slice(0, 200);
    contentArea.innerText = content;
    event.preventDefault();
  }, {once: true});

  if (!document.execCommand('paste')) {
    throw new Error('Failed to execute paste');
  }

  if (result != contentArea.innerText && contentArea.innerText != "") {
    scanButton.innerText = "Extension able to access content";
  } else {
    scanButton.innerText = "Extension couldn't access content";
  }


});
