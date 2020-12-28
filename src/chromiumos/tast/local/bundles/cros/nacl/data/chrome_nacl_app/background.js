// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Background page of the Test App.

function createModuleEmbed() {
  const moduleEmbed = document.createElement('embed');
  moduleEmbed.type = 'application/x-pnacl';
  moduleEmbed.width = 0;
  moduleEmbed.height = 0;
  moduleEmbed.src = 'nacl_module.nmf';
  return moduleEmbed;
}

function catchModuleFailures(moduleEmbed, failureCallback) {
  for (let eventType of ['abort', 'error', 'crash']) {
    moduleEmbed.addEventListener(eventType, () => {
      failureCallback(new Error(`Received "${
          eventType}" event from NaCl module: ${moduleEmbed.lastError}`));
    });
  }
}

function loadModule(moduleEmbed) {
  return new Promise((resolve, reject) => {
    moduleEmbed.addEventListener('load', () => { resolve(); });
    document.body.appendChild(moduleEmbed);
    // Request the offsetTop property to force a relayout. Without this, Chrome
    // doesn't load the module in the background page (see crbug.com/350445).
    moduleEmbed.offsetTop = moduleEmbed.offsetTop;
  });
}

async function exchangePingPongMessagesWithModule(moduleEmbed) {
  const pongPromise = getModuleMessageWaiter(moduleEmbed, 'pong');
  moduleEmbed.postMessage('ping');
  await pongPromise;
}

function getModuleMessageWaiter(moduleEmbed, expectedMessage) {
  return new Promise((resolve, reject) => {
    moduleEmbed.addEventListener('message', (message) => {
      if (message.data === expectedMessage) {
        resolve();
        return;
      }
      const formattedMessage = JSON.stringify(message.data);
      reject(`Unexpected message from NaCl module: ${formattedMessage}`);
    });
  });
}

// Entry point function. It's called by the Go counterpart of the test.
function runTest() {
  return new Promise((resolve, reject) => {
    const moduleEmbed = createModuleEmbed();
    catchModuleFailures(moduleEmbed, reject);
    loadModule(moduleEmbed)
        .then(() => { return exchangePingPongMessagesWithModule(moduleEmbed); })
        .then(resolve, reject);
  });
}
