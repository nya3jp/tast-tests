/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedkeyboardtest;

import android.app.Activity;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.os.Bundle;
import java.util.List;
import android.view.KeyEvent;
import android.view.View;
import android.widget.Button;

public class MainActivity extends Activity {
    /**
     * Represents a key that needs to be added to the view on load, and removed when the
     * corresponding keyCode is clicked.
     */
    private class KeyTestItem {
        public final Integer keyCode;
        public final String displayName;
        public final int layoutId;

        public KeyTestItem(Integer keycode, String displayName){
            this.keyCode = keycode;
            this.displayName = displayName;
            this.layoutId = View.generateViewId();
        }
    }

    /**
     * Holds all of the keys that need to be tested.
     */
    private List<KeyTestItem> keyCodesToTest = List.of(
        new KeyTestItem(KeyEvent.KEYCODE_DPAD_LEFT, "KEYS TEST - LEFT ARROW")  ,
        new KeyTestItem(KeyEvent.KEYCODE_DPAD_DOWN, "KEYS TEST - DOWN ARROW")  ,
        new KeyTestItem(KeyEvent.KEYCODE_DPAD_RIGHT, "KEYS TEST - RIGHT ARROW")  ,
        new KeyTestItem(KeyEvent.KEYCODE_DPAD_UP, "KEYS TEST - UP ARROW")  ,
        new KeyTestItem(KeyEvent.KEYCODE_TAB, "KEYS TEST - TAB")  ,
        new KeyTestItem(KeyEvent.KEYCODE_ESCAPE, "KEYS TEST - ESCAPE") ,
        new KeyTestItem(KeyEvent.KEYCODE_ENTER, "KEYS TEST - ENTER") ,
        new KeyTestItem(KeyEvent.KEYCODE_FORWARD, "KEYS TEST - FORWARD"),
        new KeyTestItem(KeyEvent.KEYCODE_BACK, "KEYS TEST - BACK")
    );

    private LinearLayout layoutMain;

    private Button btnStartKeysTest;
    private Boolean isKeysTestStarted = false;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        // Setup text views with all of the keys that will be tested.
        this.layoutMain = findViewById(R.id.layoutMain);
        for (KeyTestItem curKeyTestItem : this.keyCodesToTest){
            TextView el = new TextView(this);
            el.setId(curKeyTestItem.layoutId);
            el.setText(curKeyTestItem.displayName);
            this.layoutMain.addView(el);
        }

        // Focus the layout when the start keys button is clicked.
        this.btnStartKeysTest = (Button)findViewById(R.id.btnStartKeysTest);
        this.btnStartKeysTest.setOnClickListener(v -> {
            // Required to get focus on the layout.
            this.layoutMain.setFocusableInTouchMode(true);
            this.layoutMain.requestFocus();
            this.isKeysTestStarted = true;
        });
    }

    /**
     * Override keydown on the app itself so when the layout is focused, the keys can be
     * captured for the keys test. Note that this will not fire when an input is focused,
     * which allows other tests to run.
     *
     * At the start of the application, a series of labels for each key that should be tracked
     * will be added to the view. In this method, when a key is clicked, and the label still exists,
     * it will be deleted. This gives the testers the ability to check for existance of labels
     * to see if a key was pressed.
     * @param keyCode
     * @param event
     * @return
     */
    @Override
    public boolean onKeyDown(int keyCode, KeyEvent event) {
        // If the keys test hasn't been started, act normal.
        if(this.isKeysTestStarted != true){
            return super.onKeyDown(keyCode, event);
        }

        // Handle the case where the key pressed matches a key being looked for.
        KeyTestItem foundItem = this.keyCodesToTest.stream()
            .filter(x -> x.keyCode == keyCode)
            .findFirst()
            .orElse(null);

        if(foundItem != null){
            // Find the corresponding element and remove it if it exists.
            TextView foundViewItem = (TextView)this.layoutMain.findViewById(foundItem.layoutId);
            if(foundViewItem != null){
                this.layoutMain.removeView(foundViewItem);
            }
        }

        // Always prevent upwards propagation. This app should handle keys it wants to listen to
        // without other logic being triggered.
        return true;
    }

    /**
     * Block the back button from doing anything so it can be caught in the onKeyDown handler.
     */
    @Override
    public void onBackPressed() {
        // If the keys test hasn't been started, act normal.
        if(this.isKeysTestStarted != true){
            super.onBackPressed();
        }

        // Otherwise, do nothing.
    }
}