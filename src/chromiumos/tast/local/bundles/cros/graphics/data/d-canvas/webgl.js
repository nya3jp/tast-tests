// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
'use strict';

function initWebGL() {
  const canvas = document.querySelector('canvas');
  const gl = canvas.getContext('webgl', {desynchronized: true, alpha: false});

  gl.clearColor(1, 1, 0, 1);
  gl.clear(gl.COLOR_BUFFER_BIT);
  gl.scissor(125, 63, 250, 125);
  gl.enable(gl.SCISSOR_TEST);

  let hue = 0;
  const raf = () => {
    hue += 1;
    const r = Math.sin(0.05 * hue + 0) * 127 + 128;
    const g = Math.sin(0.05 * hue + 2) * 127 + 128;
    const b = Math.sin(0.05 * hue + 4) * 127 + 128;
    gl.clearColor(r / 255, g / 255, b / 255, 1);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.flush();
    requestAnimationFrame(raf);
  };
  requestAnimationFrame(raf);

  const setSizeAndRotation = () => {
    const angle = screen.orientation.angle % 360;
    canvas.style.transform = `rotateZ(${angle}deg)`;
    const dpr = devicePixelRatio;

    const dp_width = window.innerWidth;
    const dp_height = window.innerHeight;
    const pixel_width = Math.floor(dp_width * dpr);
    const pixel_height = Math.floor(dp_height * dpr);
    console.log('update:' + angle + ", size=" + dp_width + "x" + dp_height);

    if (angle % 180 == 90) {
      canvas.style.width = `${dp_height}px`;
      canvas.style.height = `${dp_width}px`;
      const offset = (dp_height - dp_width) / 2;
      canvas.style.left = `-${offset}px`;
      canvas.style.top = `${offset}px`;
      canvas.height = pixel_width;
      canvas.width = pixel_height;
    } else {
      canvas.style.width = `${dp_width}px`;
      canvas.style.height = `${dp_height}px`;
      canvas.style.left = "0px";
      canvas.style.top = "0px";
      canvas.width  = pixel_width;
      canvas.height = pixel_height;
    }

    gl.disable(gl.SCISSOR_TEST);
    gl.clearColor(1, 1, 0.2, 1);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.flush();
    gl.scissor(125, 63, 250, 125);
    gl.enable(gl.SCISSOR_TEST);
  };
  screen.orientation.addEventListener('change', setSizeAndRotation);
  window.addEventListener('resize',  setSizeAndRotation);
  setSizeAndRotation();
}
