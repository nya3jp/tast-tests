// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.appposition;

import android.app.Activity;
import android.os.Bundle;
import android.widget.ImageView;

import com.google.android.chromeos.activity.ChromeOsTaskManagement;

public abstract class BaseActivity extends Activity {
    private static final int ARC_SUPPORTLIB_VERSION = 1;

    ChromeOsTaskManagement mTaskManagement;
    ImageView mImage;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        getActionBar().hide();

        setContentView(R.layout.activity_main);
        mImage = findViewById(R.id.image);

        mTaskManagement = new ChromeOsTaskManagement(ARC_SUPPORTLIB_VERSION, this);
    }
}
