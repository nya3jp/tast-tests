// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Code which evaluates to |true| when the page has quiesced, i.e. has become
// dormant because no further load events have happened for a while.

// This code is used in WaitForExpr, which wraps it inside !!( ... ).
// It cannot be a plain block because JS syntax does not allow that.

(function () {

  if (document.readyState !== 'complete') {
    return false;
  }

  const QUIESCENCE_TIMEOUT_MS = 2000;
  let lastEventTime = performance.timing.loadEventEnd -
      performance.timing.navigationStart;
  const resourceTimings = performance.getEntriesByType('resource');
  if (resourceTimings.length > 0) {
    lastEntry = resourceTimings.pop();
    lastEventTime = Math.max(lastEventTime, lastEntry.responseEnd);
  }

  return performance.now() >= lastEventTime + QUIESCENCE_TIMEOUT_MS
})()
