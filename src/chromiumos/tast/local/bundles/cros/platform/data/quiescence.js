{
  var real_performance = window.performance;
  var lastEntry = null;
  var QUIESCENCE_TIMEOUT_MS = 2000;

  if (window.document.readyState !== 'complete') {
    resolve(false);
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

  resolve(lastResponseTimeMs >= QUIESCENCE_TIMEOUT_MS);
}
