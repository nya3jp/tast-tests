// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

"use strict"

function projectionMatrix(fov_y, ar, z_near, z_far){
  var temp = 1.0/ Math.tan(deg_to_rad(fov_y/2));
  return [
        temp/ar, 0,0,0,
  0,temp,0,0,
  0,0, -(z_far+z_near)/(z_far - z_near),-1,
  0,0, -2*z_near*z_far/(z_far-z_near),0
  ];
}

function createShader(gl, type, source){
  var shader = gl.createShader(type);
  gl.shaderSource(shader,source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    console.log(gl.getShaderInfoLog(shader));
  }
  return shader;
}

function onResized() {
  canvas.width = window.innerWidth;
  canvas.height = window.innerHeight;
}

function main() {
  var canvas = document.querySelector("#canvas");
     var gl = canvas.getContext("webgl");
  if (!gl)
    return;

  window.addEventListener('resize', onResized, false);
  onResized();
  var v_shader = createShader(gl,
    gl.VERTEX_SHADER,document.getElementById("vertex_shader").text);
  var f_shader = createShader(gl,
    gl.FRAGMENT_SHADER,document.getElementById("fragment_shader").text);

  var program = gl.createProgram();
  gl.attachShader(program, v_shader);
  gl.attachShader(program, f_shader);
  gl.linkProgram(program);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)){
      console.log(gl.getProgramInfoLog(program));
      gl.deleteProgram(program);
    return;
  }
      gl.useProgram(program);

    var aspect_ratio = 1.0;
  if (0 != canvas.height)
    aspect_ratio = canvas.width / canvas.height;

  var p_matrix = projectionMatrix(120, aspect_ratio, 0.01, 1000);
  var vs_proj = gl.getUniformLocation(program, "vs_proj");

  var vs_view = gl.getUniformLocation(program, "vs_view");
  var v_matrix =[1,0,0,0, 0,1,0,0, 0,0,1,0, 0,0,-15,1];

  var vs_model = gl.getUniformLocation(program, "vs_model");
  var m_matrix = [1,0,0,0, 0,1,0,0, 0,0,1,0, 0,0,0,1];

  gl.uniformMatrix4fv(vs_proj, false, p_matrix);
  gl.uniformMatrix4fv(vs_view, false, v_matrix);
  gl.uniformMatrix4fv(vs_model, false, m_matrix);

  var vs_grid_size = gl.getUniformLocation(program, "vs_grid_size");
  var grid_size = 30;
  gl.uniform1i(vs_grid_size, grid_size);

  var vertex_count = 36 * grid_size * grid_size + 6;
        var vertices = new Array(vertex_count);
        var i;
        for (i = 0; i < vertex_count; ++i) {
          vertices[i] = i;
        }
        var vertex_buffer = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, vertex_buffer);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(vertices),
          gl.STATIC_DRAW);

        var vs_vertex = gl.getAttribLocation(program, "vs_vertex");
        gl.vertexAttribPointer(vs_vertex,1,gl.FLOAT, false,0,0);
        gl.enableVertexAttribArray(vs_vertex);

  var vs_t = gl.getUniformLocation(program, "vs_t");
  var last_time = 0;
  var skip = true;

function update(time){
  // Skipping every other frames so that update is at half rate. This is to
  // lower the computational cost so low-end devices can have reasonable
  // results for performance tast tests.
  skip = !skip;
  if (skip){
    requestAnimationFrame(update);
    return;
  }

  last_time = time;

  if (0 != canvas.height)
    aspect_ratio = canvas.width / canvas.height;

  p_matrix = projectionMatrix(120, aspect_ratio, 0.01, 1000);

  gl.frontFace(gl.CW);
  gl.clearColor(0,0,0,0);
  gl.clearDepth(1);
  gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);

  gl.enable(gl.DEPTH_TEST);
  gl.depthFunc(gl.LEQUAL);
  gl.depthMask(true);
  gl.depthRange(0.0, 1.0);

  //gl.enable(gl.BLEND);
  //gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
  //gl.blendEquation(gl.FUNC_ADD);

  gl.viewport(0,0, canvas.width, canvas.height);

  gl.uniform1f(vs_t, time*0.01);
  gl.enable(gl.CULL_FACE);
  //gl.cullFace(gl.FRONT);
  //gl.drawArrays(gl.TRIANGLES,0, vertex_count);
  gl.cullFace(gl.BACK);
  gl.drawArrays(gl.TRIANGLES,0, vertex_count);
  window.requestAnimationFrame(update);
  }
  requestAnimationFrame(update);
}

function deg_to_rad(angle) {
  return angle * Math.PI / 180;
}

main();
