// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// The content here will be fed into Conn.EvalPromise().
new Promise((resolve, reject) => {
  chrome.fileSystem.getVolumeList((volumes) => {
    if (volumes) {
      for (const volume of volumes) {
        const volumeId = volume.volumeId;
        if (volumeId.indexOf('downloads:Downloads') !== -1 ||
            volumeId.indexOf('downloads:MyFiles') !== -1) {
          chrome.fileSystem.requestFileSystem(
              volume, (fs) => {
                if (fs)
                  resolve([fs.root, volumeId]);
                else
                  reject(new Error('Failed to get file system'));
              });
          return;
        }
      }
    }
    reject(new Error('Failed to get volume list'));
  });
})
.then(([rootDir, volumeId]) => {
  if (volumeId && volumeId.indexOf('downloads:MyFiles') !== -1) {
    const readDir = (dir) => {
      return !dir ? Promise.resolve([]) :
          new Promise((resolve, reject) => {
            const dirReader = dir.createReader();
            let entries = [];
            const readEntries = () => {
              dirReader.readEntries((inEntries) => {
                if (inEntries.length === 0) {
                  resolve(entries);
                  return;
                }
                entries = entries.concat(inEntries);
                readEntries();
              }, reject);
            };
            readEntries();
          });
    };
    return readDir(rootDir).then((entries) => {
      return entries.findIndex(
          (entry) => entry.name === 'Downloads' &&
          entry.isDirectory) >= 0;
    });
  }
  return dir != null;
})

