/*
 * Copyright 2022 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */
package org.chromium.arc.testapp.randomuielements;
import java.util.Random;
import android.app.Activity;
import android.os.Bundle;
import android.widget.Chronometer;
import android.widget.RadioButton;
import android.widget.Switch;
import android.widget.TextView;
import android.widget.ToggleButton;
import android.widget.CheckBox;
import android.widget.LinearLayout;
import android.view.ViewGroup.LayoutParams;
import android.widget.TableRow;
/*
 * Activity with arbitrary number of random UI elements.
 * This app can be used to test ui performance in ARC.
 * How to receive parameters for element number, how to present all of elements
 * if the number is too large is still TBD.
 */
public class RandomUIElementsActivity extends Activity{
    public final static String NUM_ELEMENTS_KEY = "num_elements";
    private final static int DEFAULT_NUM_ELEMENTS = 500;
    // TODO(sstan): Change background color for per-elements.
    private final static int BACKGROUND_COLOR = 0xff00ff00;
    private LinearLayout mLayout;
    // Use the constant seed in order to get predefined order of elements.
    private Random mRandom = new Random(0);
    @Override
    protected void onCreate(final Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.layout);
        mLayout = findViewById(R.id.root_layout);
        mLayout.setBackgroundColor(BACKGROUND_COLOR);
        // Read requested number of elements in layout.
        int numElements = getIntent().getIntExtra(NUM_ELEMENTS_KEY, DEFAULT_NUM_ELEMENTS);

        LinearLayout horizontalLayout = new LinearLayout(this);
        for (int i = 0; i < numElements; ++i) {
            if (i % 10 == 0) {
                horizontalLayout = new LinearLayout(this);
                horizontalLayout.setOrientation(LinearLayout.HORIZONTAL);
                mLayout.addView(horizontalLayout);
            }
            switch (mRandom.nextInt(6)) {
            case 0:
                horizontalLayout.addView(createRadioButton());
                break;
            case 1:
                horizontalLayout.addView(createToggleButton());
                break;
            case 2:
                horizontalLayout.addView(createSwitch());
                break;
            case 3:
                horizontalLayout.addView(createRandomTestTextView());
                break;
            case 4:
                horizontalLayout.addView(createChronometer());
                break;
            case 5:
                horizontalLayout.addView(createCheckBox());
                break;
            }
        }
        setContentView(mLayout);
    }
    private TextView createRandomTestTextView() {
        TextView textView = new TextView(this);
        StringBuffer buffer = new StringBuffer();
        buffer.append("Line:" + mRandom.nextInt());
        textView.setText(buffer);
        LayoutParams params = new TableRow.LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f);
        textView.setLayoutParams(params);
        return textView;
    }
    private RadioButton createRadioButton() {
        RadioButton button = new RadioButton(this);
        LayoutParams params = new TableRow.LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f);
        button.setLayoutParams(params);
        return button;
    }
    private ToggleButton createToggleButton() {
        ToggleButton button = new ToggleButton(this);
        button.setChecked(mRandom.nextBoolean());
        LayoutParams params = new TableRow.LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f);
        button.setLayoutParams(params);
        return button;
    }
    private Switch createSwitch() {
        Switch button = new Switch(this);
        button.setChecked(mRandom.nextBoolean());
        LayoutParams params = new TableRow.LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f);
        button.setLayoutParams(params);
        return button;
    }
    private Chronometer createChronometer() {
        Chronometer chronometer = new Chronometer(this);
        chronometer.setBase(mRandom.nextLong());
        LayoutParams params = new TableRow.LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f);
        chronometer.setLayoutParams(params);
        chronometer.start();
        return chronometer;
    }
    private CheckBox createCheckBox() {
        CheckBox checkbox = new CheckBox(this);
        checkbox.setChecked(mRandom.nextBoolean());
        LayoutParams params = new TableRow.LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f);
        checkbox.setLayoutParams(params);
        return checkbox;
    }
}