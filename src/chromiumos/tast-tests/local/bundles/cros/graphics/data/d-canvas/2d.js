// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
'use strict';

let pixel_width;
let pixel_height;

function init2D() {
  const canvas = document.querySelector('canvas');
  const container = document.querySelector('#container');

  const setSizeAndRotation = () => {
    const angle = screen.orientation.angle % 360;
    const dpr = devicePixelRatio;
    const dp_width = container.clientWidth;
    const dp_height = container.clientHeight;
    pixel_width = Math.round(dp_width * dpr);
    pixel_height = Math.round(dp_height * dpr);

    if (angle % 180 == 90) {
      canvas.style.width = `${dp_height}px`;
      canvas.style.height = `${dp_width}px`;
      const tmp = pixel_height;
      pixel_height = pixel_width;
      pixel_width = tmp;
    } else {
      canvas.style.width = `${dp_width}px`;
      canvas.style.height = `${dp_height}px`;
    }
    canvas.width  = pixel_width;
    canvas.height = pixel_height;

    canvas.style.transform = `rotateZ(${angle}deg)`;
    switch (angle) {
      case 0:
        canvas.style.left = "0px";
        canvas.style.top = "0px";
        break;
      case 90:
        canvas.style.left = `${dp_width}px`;
        canvas.style.top = "0px";
        break;
      case 180:
        canvas.style.left = `${dp_width}px`;
        canvas.style.top = `${dp_height}px`;
        break;
      case 270:
        canvas.style.left = "0px";
        canvas.style.top = `${dp_height}px`;
        break;
    }

    console.log("update1:" + angle + ", size=" + dp_width + "x" + dp_height +
                " angle=" + screen.orientation.angle);
  };
  screen.orientation.addEventListener('change', setSizeAndRotation);
  window.addEventListener('resize',  setSizeAndRotation);
  setSizeAndRotation();
  draw();
}

let deg = 0;

function draw() {
  const angle = screen.orientation.angle % 360;
  const dpr = devicePixelRatio;
  const container = document.querySelector('#container');
  const dp_width = container.clientWidth;
  const dp_height = container.clientHeight;

  const canvas = document.querySelector('canvas');
  const c2 = canvas.getContext('2d', {desynchronized: true, alpha: false});

  c2.fillStyle = 'rgb(255,255,0)';
  c2.fillRect(0, 0, pixel_width, pixel_height);
  c2.strokeStyle = 'rgb(255,0,0)';
  c2.strokeRect(0, 0, pixel_width, pixel_height);

  // Text
  c2.fillStyle = 'rgb(255,255,255)';
  c2.font = "40px Arial";
  const text = `Pixel size=${pixel_width}x${pixel_height} \
dp size=${dp_width}x${dp_height} dpr=${dpr} angle=${angle}`;
  c2.fillText(text, 10, 50);
  c2.strokeStyle = 'rgb(0,0,0)';
  c2.strokeText(text, 10, 50);

  c2.save();
  c2.translate(300, 300);
  c2.rotate(deg);
  deg += 0.02;
  c2.fillStyle = 'rgb(255,255,255)';
  c2.fillRect(-150, -150, 300, 300);
  c2.restore();

  // don't use requestAnimationFrame
  setTimeout(draw, 16);
}
