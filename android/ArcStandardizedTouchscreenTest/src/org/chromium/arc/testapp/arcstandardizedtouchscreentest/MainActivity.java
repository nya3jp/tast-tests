/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedtouchscreentest;

import android.app.Activity;
import android.widget.LinearLayout;
import android.os.Bundle;
import android.widget.Button;
import android.widget.TextView;

public class MainActivity extends Activity {

    private LinearLayout layoutMain;

    private Button btnClick;
    private int btnClickCounter = 1;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        layoutMain = findViewById(R.id.layoutMain);

        // Add the text 'Touchscreen Click' when the left click button is pressed.
        // Always add the click counter so the tast test can make sure a single click
        // doesn't fire two events.
        btnClick = findViewById(R.id.btnClick);
        btnClick.setOnClickListener(v -> {
            addTextViewToLayout(String.format("TOUCHSCREEN CLICK (%d)", btnClickCounter));
            btnClickCounter++;
        });
    }

    private void addTextViewToLayout(String text) {
        TextView el = new TextView(this);
        el.setText(text);
        layoutMain.addView(el);
    }
}