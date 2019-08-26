// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// It overrides addLegacyStats() defined in chrome://webrtc-internals.

const googMaxDecodeMs = [];
const googDecodeMs = [];

addLegacyStats = function(data) {
  const reports = data.reports;
  for (let i = 0; i < reports.length; i++) {
    if (reports[i].type === 'ssrc') {
      const values = reports[i].stats.values;
      for (let j = 0; j < values.length; j++) {
        if (values[j] === 'googMaxDecodeMs') {
          googMaxDecodeMs[googMaxDecodeMs.length] = values[j + 1];
        } else if (values[j] === 'googDecodeMs') {
          googDecodeMs[googDecodeMs.length] = values[j + 1];
        }
      }
    }
  }
};
