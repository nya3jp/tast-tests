// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

const cacheStorageKey = 'sharesheet_web_share_target';

const cacheList = [
  "sharesheet_index.html",
  "sharesheet_icon.png",
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
