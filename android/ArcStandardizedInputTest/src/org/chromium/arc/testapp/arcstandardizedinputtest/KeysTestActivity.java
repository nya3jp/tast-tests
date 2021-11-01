/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedinputtest;

import android.app.Activity;
import android.os.Bundle;
import android.view.KeyEvent;
import android.view.View;
import android.widget.LinearLayout;
import android.widget.TextView;

import java.util.ArrayList;
import java.util.List;

public class KeysTestActivity extends Activity {

    /**
     * Represents a key that needs to be added to the view on load, and removed when the
     * corresponding keyCode is clicked.
     */
    private class KeyTestItem {

        public final int keyCode;
        public final String displayName;
        public final int layoutId;

        public KeyTestItem(int keyCode, String displayName) {
            this.keyCode = keyCode;
            this.displayName = displayName;
            this.layoutId = View.generateViewId();
        }
    }

    /** Holds all of the keys that need to be tested. */
    private List<KeyTestItem> mKeyCodesToTest;

    private LinearLayout mLayout;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_keys_test);

        // Setup the keys to test.
        mKeyCodesToTest = new ArrayList<>();
        mKeyCodesToTest.add(new KeyTestItem(KeyEvent.KEYCODE_DPAD_LEFT, "KEYS TEST - LEFT ARROW"));
        mKeyCodesToTest.add(new KeyTestItem(KeyEvent.KEYCODE_DPAD_DOWN, "KEYS TEST - DOWN ARROW"));
        mKeyCodesToTest.add(
                new KeyTestItem(KeyEvent.KEYCODE_DPAD_RIGHT, "KEYS TEST - RIGHT ARROW"));
        mKeyCodesToTest.add(new KeyTestItem(KeyEvent.KEYCODE_DPAD_UP, "KEYS TEST - UP ARROW"));
        mKeyCodesToTest.add(new KeyTestItem(KeyEvent.KEYCODE_TAB, "KEYS TEST - TAB"));
        mKeyCodesToTest.add(new KeyTestItem(KeyEvent.KEYCODE_ESCAPE, "KEYS TEST - ESCAPE"));
        mKeyCodesToTest.add(new KeyTestItem(KeyEvent.KEYCODE_ENTER, "KEYS TEST - ENTER"));
        mKeyCodesToTest.add(new KeyTestItem(KeyEvent.KEYCODE_FORWARD, "KEYS TEST - FORWARD"));
        mKeyCodesToTest.add(new KeyTestItem(KeyEvent.KEYCODE_BACK, "KEYS TEST - BACK"));

        // Setup text views with all of the keys that will be tested.
        mLayout = findViewById(R.id.layoutStandardizedTest);
        for (KeyTestItem curKeyTestItem : mKeyCodesToTest) {
            TextView el = new TextView(this);
            el.setId(curKeyTestItem.layoutId);
            el.setText(curKeyTestItem.displayName);
            mLayout.addView(el);
        }

        // Force focus on the layout so the key presses can be caught.
        mLayout.setFocusableInTouchMode(true);
        mLayout.requestFocus();
    }

    /**
     * At the start of the application, a series of labels for each trackable key will be added to
     * the view. In this method, when a key is clicked, and the label still exists, it will be
     * deleted. This gives the testers the ability to check for existence of labels to see if a key
     * was pressed.
     */
    @Override
    public boolean onKeyDown(int keyCode, KeyEvent event) {
        // Handle the case where the key pressed matches a key being looked for.
        KeyTestItem foundItem =
                mKeyCodesToTest.stream().filter(x -> x.keyCode == keyCode).findFirst().orElse(null);

        if (foundItem != null) {
            // Find the corresponding element and remove it if it exists.
            TextView foundViewItem = (TextView) mLayout.findViewById(foundItem.layoutId);
            if (foundViewItem != null) {
                mLayout.removeView(foundViewItem);
            }
        }

        // Always prevent upwards propagation. This app should handle keys it wants to listen to
        // without other logic being triggered.
        return true;
    }

    /** Block the back button from doing anything so it can be caught in the onKeyDown handler. */
    @Override
    public void onBackPressed() {
        // Do nothing.
    }
}
