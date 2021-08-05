/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedmousetest;

import android.app.Activity;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.os.Bundle;
import android.widget.Button;
import android.view.MotionEvent;

public class MainActivity extends Activity {

  private LinearLayout layoutMain;
  private Button btnLeftClick;
  private Integer btnLeftClickCounter = 1;

  private Button btnRightClick;
  private Integer btnRightClickCounter = 1;

  private Button btnStartHoverTest;
  private Integer hoverOverLeaveCounter = 1;


  @Override
  protected void onCreate(Bundle savedInstanceState) {
    super.onCreate(savedInstanceState);
    setContentView(R.layout.activity_main);

    this.layoutMain = findViewById(R.id.layoutMain);

    // Add the text 'Mouse Left Click' when the left click button is pressed.
    // Always add the click counter so the tast test can make sure a single click
    // doesn't fire two events.
    this.btnLeftClick = findViewById(R.id.btnLeftClick);
    this.btnLeftClick.setOnClickListener((v) -> {
      this.addTextViewToLayout(String.format("MOUSE LEFT CLICK (%d)", this.btnRightClickCounter));
      this.btnLeftClickCounter++;
    });

    // Add the text 'Mouse Right Click' when the right click button is pressed.
    // Always add the click counter so the tast test can make sure a single click
    // doesn't fire two events.
    // 'OnContextClick' is fired natively when the mouse right clicks a button.
    this.btnRightClick = findViewById(R.id.btnRightClick);
    this.btnRightClick.setOnContextClickListener((v) -> {
      this.addTextViewToLayout(String.format("MOUSE RIGHT CLICK (%d)", this.btnRightClickCounter));
      this.btnRightClickCounter++;
      return true;
    });

    // When clicked, creates a new element that responds to the HOVER_ENTER
    // and HOVER_EXIT events by writing out a text field that can be validated
    // on the tast side.
    this.btnStartHoverTest = findViewById(R.id.btnStartHoverTest);
    this.btnStartHoverTest.setOnClickListener(v -> {
      Button el = new Button(this);
      el.setText("HOVER");
      el.setOnHoverListener((hoverView, hoverEvent) -> {
        switch (hoverEvent.getAction()) {
          case MotionEvent.ACTION_HOVER_ENTER:
            this.addTextViewToLayout(String.format("HOVER ENTER (%d)", this.hoverOverLeaveCounter));
            break;
          case MotionEvent.ACTION_HOVER_EXIT:
            this.addTextViewToLayout(String.format("HOVER EXIT (%d)", this.hoverOverLeaveCounter));
            // Add to the counter after the completion of an enter/exit pair.
            this.hoverOverLeaveCounter++;
            break;
          default:
            // Do nothing.
            break;
        }

        return true;
      });
      this.layoutMain.addView(el);
    });
  }

  private void addTextViewToLayout(String text) {
    TextView el = new TextView(this);
    el.setText(text);
    this.layoutMain.addView(el);
  }
}