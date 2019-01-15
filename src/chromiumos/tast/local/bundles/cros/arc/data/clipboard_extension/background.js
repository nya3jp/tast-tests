// Copyright 2016 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

const clipboardDump = 'clipboard_dump';

function copyTextToClipboard(src) {
  const dumpTxt = document.getElementById(clipboardDump);
  dumpTxt.value = src;
  dumpTxt.select();
  const result = document.execCommand('copy');
  dumpTxt.value = '';
  return result;
}

function pasteTextFromClipboard() {
  const dumpTxt = document.getElementById(clipboardDump);
  dumpTxt.select();
  document.execCommand('paste');
  const result = dumpTxt.value;
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
  document.addEventListener('copy', onCopy, {once: true});

  dumpTxt.value = src;
  dumpTxt.select();
  const result = document.execCommand('copy');
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
  document.addEventListener('paste', onPaste, {once: true});

  dumpTxt.select();
  document.execCommand('paste');
  const result = dumpTxt.value;
  dumpTxt.value = ''
  return result;
}

function copyImageToClipboard() {
  const imageContainer = document.getElementById("image_container");
  const range = document.createRange();
  range.selectNodeContents(imageContainer);
  const selection = window.getSelection();
  selection.removeAllRanges();
  selection.addRange(range);
  const result = document.execCommand('copy');
  return result;
}
