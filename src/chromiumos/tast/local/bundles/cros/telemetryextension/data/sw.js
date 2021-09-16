// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

chrome.runtime.onMessageExternal.addListener(
    async function(request, sender, sendResponse) {
        console.log('Received message from PWA: ', request);

        const oemData = await chrome.os.telemetry.getOemData();
        const vpdInfo = await chrome.os.telemetry.getVpdInfo();
        const availableRoutines =
            await chrome.os.diagnostics.getAvailableRoutines();

        sendResponse({
            'oemData': oemData.oemData,
            'vpdInfo': vpdInfo,
            'routines': availableRoutines.routines,
        });
    }
);
