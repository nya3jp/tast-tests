/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.chromeintentpicker;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;

public class MainActivity extends Activity {
    public static final String LOG_TAG = "CHROME_INTENT_PICKER";

    private TextView mAction;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        mAction = findViewById(R.id.intent_action);

        processIntent();
    }

    private void processIntent() {
        Log.i(LOG_TAG, "Processing intent");
        Intent intent = getIntent();
        String action = intent.getAction();

        mAction.setText(action);
    }
}
