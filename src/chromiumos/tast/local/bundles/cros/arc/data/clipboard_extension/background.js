// Copyright 2016 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

const clipboardDump = 'clipboard_dump';

function copyTextToClipboard(src) {
  const dumpTxt = document.getElementById(clipboardDump);
  dumpTxt.value = src;
  dumpTxt.select();
  result = document.execCommand('copy');
  dumpTxt.value = '';
  return result;
}

function pasteTextFromClipboard() {
  const dumpTxt = document.getElementById(clipboardDump);
  dumpTxt.select();
  document.execCommand('paste');
  result = dumpTxt.value;
  dumpTxt.value = ''
  return result;
}

function copyHtmlToClipboard(src) {
  const dumpTxt = document.getElementById(clipboardDump);

  document.addEventListener('copy', (event) => {
    const clipboardData = event.clipboardData;
    clipboardData.setData('text/html', dumpTxt.value);
    event.preventDefault();
  });

  dumpTxt.value = src;
  dumpTxt.select();
  result = document.execCommand('copy');
  dumpTxt.value = '';
  return result;
}

function pasteHtmlFromClipboard() {
  const dumpTxt = document.getElementById(clipboardDump);

  document.addEventListener('paste', (event) => {
    const clipboardData = event.clipboardData;
    dumpTxt.value = clipboardData.getData('text/html');
    event.preventDefault();
  });

  dumpTxt.select();
  document.execCommand('paste');
  result = dumpTxt.value;
  dumpTxt.value = ''
  return result;
}

