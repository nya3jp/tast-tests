// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// TabPids retrieves a list of pids for all active tabs. It returns a promise
// that resolves with the list of pids.
const TabPids = () =>
    new Promise((resolve, reject) => chrome.tabs.query({}, resolve))
        // Filter out tabs with no id, and get the internal processId of the tab
        .then(tabs => Promise.all(
            tabs.filter(tab => tab.id)
                .map(tab => new Promise((resolve, reject) =>
                    chrome.processes.getProcessIdForTab(tab.id, resolve)))))
        // Get processInfo for each processId and extract the real OS pid
        .then(ids => new Promise((resolve, reject) =>
            chrome.processes.getProcessInfo(ids, false, resolve)))
        .then(procs => Object.values(procs).map(p => p.osProcessId))
