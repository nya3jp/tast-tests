// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
    if (msg.text === "dlp") {
        document.addEventListener('paste', pasteHandler);
        if (!document.execCommand('paste')) {
            throw new Error('Failed to execute paste');
        }
    }
});


// Paste event handler
function pasteHandler(e) {
    accessTitle = "Extension Access"
    restrictedTitle = "Extension Restricted"

    result = e.clipboardData.getData("text/plain");
    // Set title.
    document.title = result ? accessTitle : restrictedTitle;

    // Prevent default paste action.
    e.preventDefault();
  }

