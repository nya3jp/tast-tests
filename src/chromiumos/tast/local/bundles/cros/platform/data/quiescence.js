// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Code which evaluates to |true| when the page has quiesced (for some unclear
// definition of "quiescing").

// This code is used in WaitForExpr, which wraps it inside !!( ... ).
// It cannot be a plain block because JS syntax does not allow that.

(function () {
  var real_performance = window.performance;
  var lastEntry = null;
  var QUIESCENCE_TIMEOUT_MS = 2000;

  if (window.document.readyState !== 'complete') {
    return false;
  }

  var resourceTimings = real_performance.getEntriesByType('resource');
  if (resourceTimings.length > 0) {
    lastEntry = resourceTimings.pop();
    real_performance.clearResourceTimings();
  }

  var timing = real_performance.timing;
  var loadTime = timing.loadEventEnd - timing.navigationStart;
  var lastResponseTimeMs = 0;

  if (!lastEntry || lastEntry.responseEnd < loadTime) {
    lastResponseTimeMs = real_performance.now() - loadTime;
  } else {
    lastResponseTimeMs = real_performance.now() - lastEntry.responseEnd;
  }

  return lastResponseTimeMs >= QUIESCENCE_TIMEOUT_MS;
})()
