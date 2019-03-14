// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function() {
function changeWindowState(predicate, getEventTarget, changeState) {
  const win = chrome.app.window.current();
  const eventTarget = getEventTarget(win);
  return new Promise((resolve) => {
    if (predicate(win)) {
      resolve();
      return;
    }
    const listener = () => {
      eventTarget.removeListener(listener);
      resolve();
    };
    eventTarget.addListener(listener);
    changeState(win);
  });
}

window.Tast = class {
  static isVideoActive() {
    const video = document.querySelector('video');
    return video && video.srcObject && video.srcObject.active;
  }

  static async restoreWindow() {
    await changeWindowState(
        (w) => !w.isMaximized() && !w.isMinimized() && !w.isFullscreen(),
        (w) => w.onRestored, (w) => w.restore());
    // Make sure it's in the foreground even if it's restored from the minimized
    // state.
    chrome.app.window.current().show();
  }

  static minimizeWindow() {
    return changeWindowState(
        (w) => w.isMinimized(), (w) => w.onMinimized, (w) => w.minimize());
  }

  static maximizeWindow() {
    return changeWindowState(
        (w) => w.isMaximized(), (w) => w.onMaximized, (w) => w.maximize());
  }

  static fullscreenWindow() {
    return changeWindowState(
        (w) => w.isFullscreen(), (w) => w.onFullscreened,
        (w) => w.fullscreen());
  }
};
})();
