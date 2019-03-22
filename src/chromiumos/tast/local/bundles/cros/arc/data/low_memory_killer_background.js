// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// promisify wraps a callback-style function into a function which returns a
// promise.
const promisify = fun => (...args) =>
    new Promise((resolve, reject) => {
        const callback = result => {
            if (chrome.runtime.lastError) {
                reject(new Error(chrome.runtime.lastError.message));
            } else {
                resolve(result);
            }
        };
        args.push(callback);
        fun.apply(null, args);
    });

const tabsQuery = promisify(chrome.tabs.query);
const getProcId = promisify(chrome.processes.getProcessIdForTab);
const getProcInfo = promisify(chrome.processes.getProcessInfo);

// TabPids retrieves a list of pids for all active tabs. It returns a promise
// that resolves with the list of pids.
async function TabPids() {
    const tabs = await tabsQuery({});
    const procIds = await Promise.all(
        tabs.filter(tab => tab.id)
            .map(tab => getProcId(tab.id)));
    const procs = await getProcInfo(procIds, false);
    return Object.values(procs).map(p => p.osProcessId);
}
