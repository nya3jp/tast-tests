// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

const TabPids = () =>
    new Promise((resolve, reject) => chrome.tabs.query({}, resolve))
        .then(tabs => Promise.all(
            tabs.filter(tab => tab.id)
                .map(tab => new Promise((resolve, reject) =>
                    chrome.processes.getProcessIdForTab(tab.id, resolve)))))
        .then(ids => new Promise((resolve, reject) =>
            chrome.processes.getProcessInfo(ids, false, resolve)))
        .then(procs => Object.values(procs).map(p => p.osProcessId))
