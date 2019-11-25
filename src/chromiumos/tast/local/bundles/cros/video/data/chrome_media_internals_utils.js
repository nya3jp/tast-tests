// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

function checkChromeMediaInternalsIsPlatformVideoDecoderForURL(theURL) {
  return new Promise((resolve, reject) => {
    const listOfPlayers = document.getElementById('player-list');
    if (listOfPlayers === null || listOfPlayers.length == 0) {
      return reject(new Error('Could not find "player-list" elements'));
    }

    const listOfItems = listOfPlayers.getElementsByTagName('li');
    if (listOfItems === null || listOfItems.length == 0) {
      return reject(
          new Error('Could not find "li" inside "player-list" element.'));
    }

    let urlFound = false;
    for (const item of listOfItems) {
      const playerName = item.getElementsByClassName('player-name');
      if (playerName === undefined || playerName.length == 0) {
        continue;
      }

      if (playerName[0].innerText.includes(theURL)) {
        urlFound = true;
        // Simulate a click to open the log for the player item.
        playerName[0].click();
        break;
      }
    }
    if (!urlFound) {
      console.error(theURL + ' url not found');
      return reject(new Error(
          theURL + ' url was not found in chrome://media-internals.'));
    }

    const logTable = document.getElementById('log');
    if (logTable === null) {
      return reject(new Error('Could not find the "log" table.'));
    }

    const logTableRow = logTable.getElementsByTagName('tr');
    if (logTableRow === null || logTableRow.length == 0) {
      return reject(new Error('Could not find log rows.'));
    }

    for (const logTableEntry of logTableRow) {
      if (logTableEntry.cells[1].innerHTML == 'is_platform_video_decoder' ||
          // Changed after crrev.com/c/1904341 (Chromium 80.0.3974.0).
          logTableEntry.cells[1].innerHTML == 'kIsPlatformVideoDecoder') {
        return resolve(logTableEntry.cells[2].innerHTML == 'true');
      }
    }
    reject(new Error('Did not find is_platform_video_decoder.'));
  });
}
