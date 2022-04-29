//Copyright 2022 The Chromium OS Authors. All rights reserved.
//Use of this source code is governed by a BSD-style license that can be
//found in the LICENSE file.

function onLoad() {
  // This cookie is a first-party cookie. The third-party cookie will be set
  // via the header of the HTTPS response.
  var cookieString = "coooookie=tasty";
  document.cookie = cookieString;
}