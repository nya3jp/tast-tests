// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

function setTitle(capture) {
  let title = "Screen capture allowed";
  if (!capture) {
    if (chrome.runtime.lastError) {
      title = chrome.runtime.lastError.message;
    } else {
      title = "Unknown error";
    }
  }

  chrome.tabs.executeScript({code: 'document.title = "' + title + '"'});
}

chrome.commands.onCommand.addListener((command) => {
  if (command === 'takeScreenshot') {
    chrome.tabs.query({active: true, currentWindow: true}, (tabs) => {
      chrome.tabs.sendMessage(tabs[0].id, {text: 'title'}, (method) => {
        if (method === 'captureVisibleTab') {
          chrome.tabs.captureVisibleTab((img) => {
            setTitle(img);
          });
        } else if (method === 'tabCapture') {
          chrome.tabCapture.capture({video: true}, (stream) => {
            setTitle(stream);
          });
        } else if (method === 'desktopCapture') {
          chrome.desktopCapture.chooseDesktopMedia(
              ['screen', 'window', 'tab'], (streamId) => {
                setTitle(streamId);
              });
        }
      });
    });
  }
});
