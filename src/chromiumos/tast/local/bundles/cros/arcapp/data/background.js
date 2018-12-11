// Copyright 2016 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// text/plain tests
function copy_text_to_clipboard(src) {
    var dump_txt = document.getElementById("clipboard_dump");
    dump_txt.value = src;
    dump_txt.select();
    result = document.execCommand("copy");
    dump_txt.value = "";
    return result;
}

function paste_text_from_clipboard() {
    var dump_txt = document.getElementById("clipboard_dump");
    dump_txt.select();
    document.execCommand("paste");
    result = dump_txt.value;
    dump_txt.value = ""
    return result;
}

function copy_html_to_clipboard(src) {
    var dump_txt = document.getElementById("clipboard_dump");

    document.addEventListener('copy', function(event) {
      var clipboardData = event.clipboardData;
      clipboardData.setData('text/html', dump_txt.value);
      event.preventDefault();
    });

    dump_txt.value = src;
    dump_txt.select();
    result = document.execCommand("copy");
    dump_txt.value = "";
    return result
}

function paste_html_from_clipboard() {
    var dump_txt = document.getElementById("clipboard_dump");

    document.addEventListener('paste', function(event) {
      var clipboardData = event.clipboardData;
      dump_txt.value = clipboardData.getData('text/html');
      event.preventDefault();
    });

    dump_txt.select();
    document.execCommand("paste");
    result = dump_txt.value;
    dump_txt.value = ""
    return result;
}

