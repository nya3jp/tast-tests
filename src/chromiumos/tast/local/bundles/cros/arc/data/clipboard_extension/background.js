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

  const onCopy = (event) => {
    const clipboardData = event.clipboardData;
    clipboardData.setData('text/html', dumpTxt.value);
    event.preventDefault();
  };
  document.addEventListener('copy', onCopy);

  dumpTxt.value = src;
  dumpTxt.select();
  result = document.execCommand('copy');
  document.removeEventListener('copy', onCopy);
  dumpTxt.value = '';
  return result;
}

function pasteHtmlFromClipboard() {
  const dumpTxt = document.getElementById(clipboardDump);

  const onPaste = (event) => {
    const clipboardData = event.clipboardData;
    dumpTxt.value = clipboardData.getData('text/html');
    event.preventDefault();
  };
  document.addEventListener('paste', onPaste);

  dumpTxt.select();
  document.execCommand('paste');
  document.removeEventListener('paste', onPaste);
  result = dumpTxt.value;
  dumpTxt.value = ''
  return result;
}

function copyImageToClipboard() {
  const imageContainer = document.getElementById("image_container");
  var range = document.createRange();
  range.selectNodeContents(imageContainer);
  var selection = window.getSelection();
  selection.removeAllRanges();
  selection.addRange(range);
  result = document.execCommand('copy');
  return result;
}


