// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

const N_latency = 100;
const sec_perf = 10;
const scheduled_perf = 20000;

async function vpdInfoLatency() {
  sum = 0.0;
  for (let i = 0; i < N_latency; i++) {
    const t0 = performance.now();
    await chrome.os.telemetry.getVpdInfo();
    const t1 = performance.now();
    sum += t1 - t0;
  }
  return sum / N_latency;
}

async function vpdInfoPerformance_old() {
  const tMs = sec_perf * 1000;
  n = 0;
  const tEnd = performance.now() + tMs;
  for (; performance.now() < tEnd; n++) {
    chrome.os.telemetry.getVpdInfo();
  }
  return n / sec_perf;
}

async function vpdInfoPerformance() {
  const tMs = sec_perf * 1000;
  scheduled = 0;
  succeded = 0;
  failed = 0;
  before_started = 0;

  started = false;
  tEnd = performance.now() + tMs;

  tBeforeLoop = performance.now();

  allPromises = [];
  for (; scheduled < scheduled_perf; scheduled++) {
    p = new Promise((resolve) => {
      chrome.os.telemetry.getVpdInfo().then(
        () => {
          if (started == false) {
            before_started++;
          } else if (performance.now() < tEnd) {
            succeded++;
          } else {
            failed++;
          }
          resolve();
        }
      );
    });
    allPromises.push(p);
  }

  started = true;
  tEnd = performance.now() + tMs;

  tBeforePromiseAll = performance.now();

  await Promise.all(allPromises);

  tAfterPromiseAll = performance.now();

  return {
    "scheduledWanted": scheduled_perf,
    "scheduled": scheduled,
    "succeded": succeded,
    "failed": failed,
    "before_started": before_started,
    "time_to_schedule": tBeforePromiseAll - tBeforeLoop,
    "time_to_wait_all": tAfterPromiseAll - tBeforePromiseAll,
  }
}

async function availableRoutinesLatency() {
  sum = 0.0;
  for (let i = 0; i < N_latency; i++) {
    const t0 = performance.now();
    await chrome.os.diagnostics.getAvailableRoutines();
    const t1 = performance.now();
    sum += t1 - t0;
  }
  return sum / N_latency;
}

async function availableRoutinesPerformance() {
  const tMs = sec_perf * 1000;
  n = 0;
  const tEnd = performance.now() + tMs;
  for (; performance.now() < tEnd; n++) {
    chrome.os.diagnostics.getAvailableRoutines();
  }
  return n / sec_perf;
}

async function permissionsLatency() {
  sum = 0.0;
  for (let i = 0; i < N_latency; i++) {
    const t0 = performance.now();
    await new Promise((resolve) => {
      chrome.permissions.contains(
        { permissions: ["os.telemetry.serial_number"] }, resolve);
    });
    const t1 = performance.now();
    sum += t1 - t0;
  }
  return sum / N_latency;
}

async function permissionsPerformance_old() {
  const tMs = sec_perf * 1000;
  n = 0;
  const tEnd = performance.now() + tMs;
  for (; performance.now() < tEnd; n++) {
    chrome.permissions.contains(
      { permissions: ["os.telemetry.serial_number"] }, () => {});
  }
  return n / sec_perf;
}

async function permissionsPerformance() {
  const tMs = sec_perf * 1000;
  scheduled = 0;
  succeded = 0;
  failed = 0;
  before_started = 0;

  started = false;
  tEnd = performance.now() + tMs;

  tBeforeLoop = performance.now();

  allPromises = [];
  for (; scheduled < scheduled_perf; scheduled++) {
    p = new Promise((resolve) => {
      chrome.permissions.contains(
        { permissions: ["os.telemetry.serial_number"] },
        () => {
          if (started == false) {
            before_started++;
          } else if (performance.now() < tEnd) {
            succeded++;
          } else {
            failed++;
          }
          resolve();
        }
      );
    });
    allPromises.push(p);
  }

  started = true;
  tEnd = performance.now() + tMs;

  tBeforePromiseAll = performance.now();

  await Promise.all(allPromises);

  tAfterPromiseAll = performance.now();

  return {
    "scheduledWanted": scheduled_perf,
    "scheduled": scheduled,
    "succeded": succeded,
    "failed": failed,
    "before_started": before_started,
    "time_to_schedule": tBeforePromiseAll - tBeforeLoop,
    "time_to_wait_all": tAfterPromiseAll - tBeforePromiseAll,
  }
}

chrome.runtime.onMessageExternal.addListener(
  async function(request, sender, sendResponse) {
    console.log('Received message from PWA: ', request);

    try {
      hasSNPermission = await new Promise((resolve) => {
        chrome.permissions.contains(
          { permissions: ["os.telemetry.serial_number"] }, resolve);
      });

      var oemData = {oemData: ''};
      if (hasSNPermission) {
        oemData = await chrome.os.telemetry.getOemData();
      }
      const vpdInfo = await chrome.os.telemetry.getVpdInfo();
      const availableRoutines =
        await chrome.os.diagnostics.getAvailableRoutines();

      stats = [
        {
          "name": "vpdInfo",
          "latency": await vpdInfoLatency(),
          "performance": await vpdInfoPerformance(), 
        },
        // {
        //   "name": "availableRoutine",
        //   // "latency": await availableRoutinesLatency(),
        //   "performance": await availableRoutinesPerformance(), 
        // },
        {
          "name": "permissionsContains",
          "latency": await permissionsLatency(),
          "performance": await permissionsPerformance(), 
        },
      ];
        
      sendResponse({
        'apiStats': JSON.stringify(stats),
        'oemData': oemData.oemData,
        'vpdInfo': vpdInfo,
        'routines': availableRoutines.routines,
      });

    } catch (error) {
      sendResponse({
        'error': error.toString(),
      });
    }
  }
);
