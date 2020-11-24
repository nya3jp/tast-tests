// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

chrome.app.window.create("scan.html", {
  singleton: true,
  id: "ChromeApps-Sample-Document-Scan",
  bounds: {
   'width': 480,
   'height': 640
  },
  alwaysOnTop: true,
  focused: true,
});
