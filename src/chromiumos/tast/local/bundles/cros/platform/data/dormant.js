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
  // Each resourceTimings element contains the load start time and load end time
  // for a resource.  If a load has not completed yet, the end time is set to
  // the current time.  Then we can tell that a load has completed by detecting
  // that the end time diverges from the current time.
  //
  // resourceTimings is sorted by event start time, so we need to look through
  // the entire array to find the latest activity.
  const maxEndTime = (current, timing) => Math.max(timing.responseEnd, current);
  lastEventTime = resourceTimings.reduce(maxEndTime, lastEventTime);
  return performance.now() >= lastEventTime + QUIESCENCE_TIMEOUT_MS;
})()
