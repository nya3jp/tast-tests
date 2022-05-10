/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.intent;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;
import android.widget.TextView;

import org.chromium.arc.testapp.intent.TestParcelable;

public class SecondActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.second_activity);

        final Intent launchIntent = getIntent();
        int extraValue = launchIntent.getIntExtra("int", -1);
        String extraText = launchIntent.getStringExtra("string");
        TestParcelable parcelable = launchIntent.getParcelableExtra("parcelable");

        TextView intView = findViewById(R.id.int_extra);
        intView.setText(String.valueOf(extraValue));
        TextView stringView = findViewById(R.id.string_extra);
        stringView.setText(extraText);
        TextView parcelView = findViewById(R.id.parcel_extra);
        parcelView.setText(parcelable.mText);
    }
}
