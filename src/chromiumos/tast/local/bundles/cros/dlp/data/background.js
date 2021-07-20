// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

  chrome.commands.onCommand.addListener((command) => {
    if (command === 'copy') {
        chrome.tabs.query({active: true, currentWindow: true}, (tabs) => {
            chrome.tabs.sendMessage(tabs[0].id, {text: 'title'}, (method) => {

                        console.log(method)

            });
        });
    }
  });