// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.appposition;

import android.graphics.drawable.ColorDrawable;
import android.os.Bundle;

public class MainActivity extends BaseActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        mImage.setImageDrawable(new ColorDrawable(getColor(R.color.foreground)));

        mTaskManagement.activateTask();
    }
}
