/*
 * Copyright 2022 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.randomelements;

import java.util.Random;

import android.app.Activity;
import android.os.Bundle;
import android.view.ViewTreeObserver.OnPreDrawListener;
import android.widget.Chronometer;
import android.widget.RadioButton;
import android.widget.Switch;
import android.widget.TextView;
import android.widget.ToggleButton;
import android.widget.CheckBox;
import android.widget.LinearLayout;

/*
 * Activity with arbitrary number of random UI elements, refresh itself constantly.
 */
public class RandomUIElementsActivity extends Activity{
    public final static String NUM_ELEMENTS_KEY = "num_elements";

    private final static int DEFAULT_NUM_ELEMENTS = 100;
    private final static int BACKGROUND_COLOR = 0xff00ff00;

    private LinearLayout mLayout;

    // Use the constant seed in order to get predefined order of elements.
    private Random mRandom = new Random(0);

    @Override
    protected void onCreate(final Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.flow_layout);

        mLayout = (LinearLayout)findViewById(R.id.root_flow_layout);
        mLayout.setBackgroundColor(BACKGROUND_COLOR);

        // Read requested number of elements in layout.
        int numElements = getIntent().getIntExtra(NUM_ELEMENTS_KEY, DEFAULT_NUM_ELEMENTS);

        for (int i = 0; i < numElements; ++i) {
            switch (mRandom.nextInt(6)) {
            case 0:
                mLayout.addView(createRadioButton());
                break;
            case 1:
                mLayout.addView(createToggleButton());
                break;
            case 2:
                mLayout.addView(createSwitch());
                break;
            case 3:
                mLayout.addView(createRandomTestTextView());
                break;
            case 4:
                mLayout.addView(createChronometer());
                break;
            case 5:
                mLayout.addView(createCheckBox());
                break;
            }
        }

        setContentView(mLayout);
    }

    private TextView createRandomTestTextView() {
        TextView textView = new TextView(this);
        int lineCnt = mRandom.nextInt(4);
        StringBuffer buffer = new StringBuffer();
        for (int i = 0; i < lineCnt; ++i) {
            if (i != 0) {
                buffer.append("\n");
            }
            buffer.append("Line:" + mRandom.nextInt());
        }
        textView.setText(buffer);
        return textView;
    }

    private RadioButton createRadioButton() {
        RadioButton button = new RadioButton(this);
        button.setText("RadioButton:" + mRandom.nextInt());
        return button;
    }

    private ToggleButton createToggleButton() {
        ToggleButton button = new ToggleButton(this);
        button.setChecked(mRandom.nextBoolean());
        return button;
    }

    private Switch createSwitch() {
        Switch button = new Switch(this);
        button.setChecked(mRandom.nextBoolean());
        return button;
    }

    private Chronometer createChronometer() {
        Chronometer chronometer = new Chronometer(this);
        chronometer.setBase(mRandom.nextLong());
        chronometer.start();
        return chronometer;
    }

    private CheckBox createCheckBox() {
        CheckBox checkbox = new CheckBox(this);
        checkbox.setChecked(mRandom.nextBoolean());
        return checkbox;
    }

}