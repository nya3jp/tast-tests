// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import android.app.Activity;
import android.os.Bundle;
import android.widget.EditText;

/**
 * Activity for ChromeOS ARC++/ARCVM tast.
 */
public class MainActivity extends Activity {

    private EditText mTestResult;
    private EditText mTestResultLog;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        mTestResult = (EditText) findViewById(R.id.test_result);
        mTestResultLog = (EditText) findViewById(R.id.test_result_log);
    }

    @Override
    protected void onStart() {
        super.onStart();
    }

    protected void markAsPassed() {
        mTestResult.setText("1");
    }

    protected void markAsFailed(String log) {
        mTestResult.setText("0");
        mTestResultLog.setText(log);
    }
}
