/*
 * Copyright 2020 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

window.addEventListener('load', () => {
  document.querySelector('#drag-start-area').addEventListener(
      'dragstart', (event) => {
    event.dataTransfer.setData('text/plain', 'Data text');
  });
}, {once: true});
