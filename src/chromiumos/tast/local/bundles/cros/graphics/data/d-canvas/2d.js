// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

var pixel_width;
var pixel_height;

function init2D() {
  let canvas = document.querySelector('canvas');

  let setSizeAndRotation = () => {
    let angle = screen.orientation.angle % 360;
    let dpr = devicePixelRatio;
    let dp_width = window.innerWidth;
    let dp_height = window.innerHeight;
    pixel_width = Math.round(dp_width * dpr);
    pixel_height = Math.round(dp_height * dpr);

    if (angle % 180 == 90) {
      canvas.style.width = `${dp_height}px`;
      canvas.style.height = `${dp_width}px`;
      var tmp = pixel_height;
      pixel_height = pixel_width;
      pixel_width = tmp;
    } else {
      canvas.style.width = `${dp_width}px`;
      canvas.style.height = `${dp_height}px`;
    }
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

    canvas.width  = pixel_width;
    canvas.height = pixel_height;

    console.log("update1 size=" + window.innerWidth + "x" + window.innerHeight +
                " angle=" + screen.orientation.angle);
  };

  screen.orientation.addEventListener('change', setSizeAndRotation);
  window.addEventListener('resize', setSizeAndRotation);

  document.documentElement.addEventListener('click',  setSizeAndRotation);
  setSizeAndRotation();
  draw();
}

var deg = 0;

function draw() {
  let angle = screen.orientation.angle % 360;
  let dpr = devicePixelRatio;
  let dp_width = window.innerWidth;
  let dp_height = window.innerHeight;

  let canvas = document.querySelector('canvas');
  let c2 = canvas.getContext('2d', {desynchronized: true, alpha: false});

  c2.fillStyle = 'rgb(255,255,0)';
  c2.fillRect(0, 0, pixel_width, pixel_height);
  c2.strokeStyle = 'rgb(255,0,0)';
  c2.strokeRect(0, 0, pixel_width, pixel_height);

  // Text
  c2.fillStyle = 'rgb(255,255,255)';
  c2.font = "40px Arial";
  var text = `Pixel size=${pixel_width}x${pixel_height} \
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
  setTimeout(draw, 166);
}
