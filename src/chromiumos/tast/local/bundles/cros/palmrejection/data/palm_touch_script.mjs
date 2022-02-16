// Modified from https://github.com/tbuckley/tbuckley.github.io/blob/main/palm/main.mjs
const preview = document.querySelector("#preview");
const canvas = preview.querySelector("canvas");
const clear = document.querySelector("#clear");
const accept = document.querySelector("#accept");
const cancel = document.querySelector("#cancel");

const ctx = canvas.getContext("2d");

const cr = preview.getBoundingClientRect();
canvas.style.width = `${cr.width}px`;
canvas.style.height = `${cr.height}px`;
canvas.width = cr.width * devicePixelRatio;
canvas.height = cr.height * devicePixelRatio;

ctx.scale(devicePixelRatio, devicePixelRatio);

clear.addEventListener("click", function() {
    ctx.clearRect(0,0,cr.width,cr.height);
});

function drawLine(a, b) {
    const cr = preview.getBoundingClientRect();
    ctx.beginPath();
    ctx.strokeStyle = "#000";
    ctx.lineWidth = 1;
    ctx.moveTo(a.clientX-cr.left, a.clientY-cr.top);
    ctx.lineTo(b.clientX-cr.left, b.clientY-cr.top);
    ctx.stroke();
}
function drawTouch(color, width, e) {
    const cr = preview.getBoundingClientRect();
    ctx.beginPath();
    ctx.strokeStyle = color;
    ctx.lineWidth = width;
    ctx.ellipse(e.clientX - cr.left,e.clientY-cr.top,e.width,e.height,0,0,Math.PI*2);
    ctx.stroke();
}

const pointers = {};

preview.addEventListener("pointerdown", function(e) {
  e.preventDefault();
  if(e.pointerType === "touch" || e.pointerType === "pen") {
    pointers[e.pointerId] = e;
    drawTouch("rgba(0,200,100,1)", 3, e);
  }
});

preview.addEventListener("pointermove", function(e) {
  if(e.pointerId in pointers) {
    drawLine(pointers[e.pointerId], e);
    drawTouch("rgba(255,0,0,1)", 1, e);
    pointers[e.pointerId] = e;
  }
});

preview.addEventListener("pointerup", function(e) {
  if(e.pointerId in pointers) {
    drawLine(pointers[e.pointerId], e);
    drawTouch("rgba(100,100,255,1)", 3, e);
    delete pointers[e.pointerId];
  }
  let span = document.createElement('span')
  span.className = 'accept'
  accept.append(span)
});

preview.addEventListener("pointercancel", function(e) {
  if(e.pointerId in pointers) {
    drawLine(pointers[e.pointerId], e);
    drawTouch("rgba(0,0,0,1)", 3, e);
    delete pointers[e.pointerId];
  }
  let span = document.createElement('span')
  span.className = 'cancel'
  cancel.append(span)
});

preview.addEventListener("contextmenu", e => e.preventDefault());
