/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedmousetest;

import android.app.Activity;
import android.os.Bundle;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

public class MainActivity extends Activity {

    private LinearLayout layoutMain;
    private Button btnLeftClick;
    private int btnLeftClickCounter = 1;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        layoutMain = findViewById(R.id.layoutMain);

        // Add the text 'Mouse Left Click' when the left click button is pressed.
        // Always add the click counter so the tast test can make sure a single click
        // doesn't fire two events.
        btnLeftClick = findViewById(R.id.btnLeftClick);
        btnLeftClick.setOnClickListener(
                (v) -> {
                    TextView el = new TextView(this);
                    el.setText(String.format("MOUSE LEFT CLICK (%d)", btnLeftClickCounter));
                    btnLeftClickCounter++;
                    layoutMain.addView(el);
                });
    }
}
