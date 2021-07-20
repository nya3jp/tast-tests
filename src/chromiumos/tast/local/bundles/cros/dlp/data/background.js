// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

chrome.app.window.create("dlp_clipboard.html", {
    innerBounds: {
     'width': 480,
     'height': 640
    },
    alwaysOnTop: true,
    focused: true,
  });