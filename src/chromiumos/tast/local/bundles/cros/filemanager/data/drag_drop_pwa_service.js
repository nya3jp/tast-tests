// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

const cacheStorageKey = 'drag_drop_pwa_key';

const cacheList = [
  "drag_drop_pwa_window.html",
  "drag_drop_pwa_icon.png",
];

self.addEventListener('install', function(e) {
  e.waitUntil(
    caches.open(cacheStorageKey).then(function(cache) {
      return cache.addAll(cacheList)
    }).then(function() {
      return self.skipWaiting()
    })
  )
})

self.addEventListener('activate', event => {
  event.waitUntil(clients.claim());
});

self.addEventListener('fetch', function(e) {
  console.log('Fetch event:', e.request)
})
