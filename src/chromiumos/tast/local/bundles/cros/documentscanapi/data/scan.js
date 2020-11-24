// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

var scanButton = document.getElementById('scanButton');
var scannedImage = document.getElementById('scannedImage');
var waitAnimation = document.getElementById('waitAnimation');
var scanCompleteText = document.getElementById('scanCompleteText');

function setOnlyChild(parent, child) {
  while (parent.firstChild) {
    parent.removeChild(parent.firstChild);
  }
  parent.appendChild(child);
}

var onScanCompleted = function(scan_results) {
  waitAnimation.style.display = 'none';
  if (chrome.runtime.lastError) {
    console.log('Scan failed: ' + chrome.runtime.lastError.message);
    return;
  }
  numImages = scan_results.dataUrls.length;
  console.log('Scan completed with ' + numImages + ' images.');
  urlData = scan_results.dataUrls[0]
  console.log('Scan data length ' +
              urlData.length + '.');
  console.log('URL is ' + urlData);
  scannedImage.src = urlData;
  scanCompleteText.value = 'Complete!';
};

scanButton.addEventListener('click', function() {
  var scanProperties = {};
  waitAnimation.style.display = 'block';
  chrome.documentScan.scan(scanProperties, onScanCompleted);
});
