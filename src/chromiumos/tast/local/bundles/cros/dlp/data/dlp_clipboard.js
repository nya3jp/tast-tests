// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

var contentArea = document.getElementById('drag-drop');
var scanButton = document.getElementById('scanButton');
scanButton.addEventListener('click', function() {
  let result = contentArea.innerText;
//   document.addEventListener('paste', (event) => {
//     // event.preventDefault();
//     let content = event.clipboardData.getData('text/plain').slice(0, 200);
//     contentArea.innerText = content;
//     event.preventDefault();
//   });
//   if (!document.execCommand('paste')) {
//     throw new Error('Failed to execute paste');
//   }

    var selectedText = window.getSelection().toString();

    contentArea.innerText = selectedText.slice(0, 200);
  if (result != contentArea.innerText && contentArea.innerText != "") {
    scanButton.innerText = "Extension able to access content";
  } else {
    scanButton.innerText = "Extension couldn't access content";
  }
});

// scanButton.addEventListener('click', () => {
//   });

//   scanButton.addEventListener('click', function () {
//     foo();
//   });

//   async function foo() {
//     let result = contentArea.innerText;
//     navigator.clipboard.readText()
//       .then(text => {
//         contentArea.innerText = text;
//         if (result != contentArea.innerText && contentArea.innerText != "") {
//             scanButton.innerText = "Extension able to access content";
//           } else {
//             scanButton.innerText = "Extension couldn't access content";
//           }
//       })
//       .catch(err => {
//         contentArea.innerText ="ERR";
//       })


//   }