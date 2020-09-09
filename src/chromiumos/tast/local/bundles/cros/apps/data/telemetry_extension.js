// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

chromeos.telemetry
    .probeTelemetryInfo([
      'battery', 'non-removable-block-devices', 'cached-vpd-data', 'cpu',
      'timezone', 'memory', 'backlight', 'fan', 'stateful-partition',
      'bluetooth'
    ])
    .then(response => {
      if (response &&
          (response.batteryResult && response.batteryResult.batteryInfo) &&
          (response.blockDeviceResult &&
           response.blockDeviceResult.blockDeviceInfo) &&
          (response.vpdResult && response.vpdResult.vpdInfo) &&
          (response.cpuResult && response.cpuResult.cpuInfo) &&
          (response.timezoneResult && response.timezoneResult.timezoneInfo) &&
          (response.memoryResult && response.memoryResult.memoryInfo) &&
          (response.backlightResult &&
           response.backlightResult.backlightInfo) &&
          (response.fanResult && response.fanResult.fanInfo) &&
          (response.statefulPartitionResult &&
           response.statefulPartitionResult.partitionInfo) &&
          (response.bluetoothResult &&
           response.bluetoothResult.bluetoothAdapterInfo)) {
        document.getElementById('telemetryApiStatus').name = 'Success';
        document.getElementById('telemetryApiStatus').textContent = 'Success';
      } else {
        document.getElementById('telemetryApiStatus').textContent = 'Failure';
      }

      document.getElementById('telemetryApiResponse').textContent =
          JSON.stringify(response, null, 2);
    })
    .catch(err => {
      document.getElementById('telemetryApiStatus').textContent = 'Failure';
      document.getElementById('telemetryApiResponse').textContent =
          JSON.stringify(err, null, 2);
    });
