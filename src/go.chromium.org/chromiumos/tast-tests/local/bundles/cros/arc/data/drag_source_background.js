/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

chrome.app.window.create("window.html", {
  outerBounds: {
    width: 500,
    height: 500,
    left: 500,
    top: 0
  },
  alwaysOnTop: true,
});
