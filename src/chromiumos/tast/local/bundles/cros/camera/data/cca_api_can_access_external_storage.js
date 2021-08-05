// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// The content here will be fed into Conn.Eval().
(function() {
  // Returns a promise containing all the entries of a given directory.
  const readDir = (dir) => {
    return new Promise((resolve, reject) => {
      const dirReader = dir.createReader();
      let entries = [];
      const readEntries = () => {
        dirReader.readEntries((inEntries) => {
          if (inEntries.length === 0) {
            resolve(entries);
            return;
          }
          entries = entries.concat(inEntries);
          // We need to call readEntries() until it returns empty array
          // because it might return partial results.
          readEntries();
        }, reject);
      };
      readEntries();
    });
  };

  const getLocalVolumeRootDir = (resolve, reject) => {
    chrome.fileSystem.getVolumeList((volumes) => {
      if (!volumes) {
        reject(new Error('Failed to get volume list'));
        return;
      }
      for (const volume of volumes) {
        const {volumeId} = volume;
        if (volumeId.includes('downloads:MyFiles')) {
          chrome.fileSystem.requestFileSystem(
              volume, (fs) => {
                if (fs) {
                  resolve([fs.root, volumeId]);
                } else {
                  reject(new Error('Failed to get file system'));
                }
              });
          return;
        }
      }
      reject(new Error('No local volume found'));
    });
  };
  return new Promise(getLocalVolumeRootDir);
})()
