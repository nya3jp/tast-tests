/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.intent;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.os.Bundle;
import android.os.Parcel;
import android.os.Parcelable;

import org.chromium.arc.testapp.intent.TestParcelable;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        findViewById(R.id.button).setOnClickListener(
                view -> MainActivity.this.startActivity(createIntent(MainActivity.this)));
    }

    public static Intent createIntent(Context context) {
        Intent launchIntent = new Intent();
        launchIntent.setClass(context, SecondActivity.class);
        launchIntent.putExtra("int", 103);
        launchIntent.putExtra("string", "test");
        launchIntent.putExtra("parcelable", new TestParcelable("parcelable test"));
        return launchIntent;
    }
}
