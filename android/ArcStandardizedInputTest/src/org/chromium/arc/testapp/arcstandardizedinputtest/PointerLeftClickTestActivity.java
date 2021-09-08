/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedinputtest;

import android.app.Activity;
import android.os.Bundle;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

public class PointerLeftClickTestActivity extends Activity {

    private LinearLayout mLayoutMain;
    private int mBtnLeftClickCounter = 1;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_pointer_left_click_test);

        mLayoutMain = findViewById(R.id.layoutMain);

        // Add the text 'Pointer Left Click' when the left click button is pressed.
        // Always add the click counter so the tast test can make sure a single click
        // doesn't fire two events.
        Button btnLeftClick = findViewById(R.id.btnLeftClick);
        btnLeftClick.setOnClickListener(
                (v) -> {
                    TextView el = new TextView(this);
                    el.setText(String.format("POINTER LEFT CLICK (%d)", mBtnLeftClickCounter));
                    mBtnLeftClickCounter++;
                    mLayoutMain.addView(el);
                });
    }
}
