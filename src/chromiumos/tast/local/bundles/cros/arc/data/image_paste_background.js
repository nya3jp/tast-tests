/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

chrome.app.window.create("foreground.html", {
  outerBounds: {
    width: 100,
    height: 100,
    left: 0,
    top: 0
  },
});
