// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

onClick = async function() {
    const image = await fetch('sample.png');
    const blob = await image.blob();
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]);
};

document.addEventListener("DOMContentLoaded", function() {
    document.querySelector('.copy_button').addEventListener('click', onClick);
});
