// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
    if (msg.text === "copy") {
        document.addEventListener('copy', copyHandler);
        if (!document.execCommand('copy')) {
            throw new Error('Failed to execute paste');
        }
    }
});


// copy event handler
function copyHandler(e) {
    var selection = document.getSelection();

    // set clipboard string.
    e.clipboardData.setData(
      'text/plain',
      selection.toString()
    );
    // stop default copy event.
    e.preventDefault();

    alertString = "Extension able to access content\n"
    if (selection.toString() == "") {
        alertString = "Extension couldn't access content\n"
    }

    alert(alertString + e.clipboardData.getData('text/plain').slice(0,30));
  }