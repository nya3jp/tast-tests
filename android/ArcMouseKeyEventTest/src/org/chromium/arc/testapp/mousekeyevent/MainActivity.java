/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.mousekeyevent;

import android.app.Activity;
import android.os.Bundle;
import android.view.KeyEvent;
import android.widget.TextView;

public class MainActivity extends Activity {
    private TextView mText;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        mText = findViewById(R.id.generated_key_events);
    }

    @Override
    public boolean onKeyDown(int keyCode, KeyEvent event) {
        mText.append("ACTION_DOWN : " + getKeyCodeName(keyCode) + "\n");
        return true;
    }

    @Override
    public boolean onKeyUp(int keyCode, KeyEvent event) {
        mText.append("ACTION_UP : " + getKeyCodeName(keyCode) + "\n");
        return true;
    }

    private String getKeyCodeName(int keyCode) {
        switch (keyCode) {
            case KeyEvent.KEYCODE_BACK:
                return "KEYCODE_BACK";
            case KeyEvent.KEYCODE_FORWARD:
                return "KEYCODE_FORWARD";
            default:
                return "Unregistered key code: " + keyCode;
        }
    }
}
