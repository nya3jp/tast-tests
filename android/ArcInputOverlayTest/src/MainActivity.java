/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputoverlay;

import android.app.Activity;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;
import android.view.InputDevice;
import android.view.InputEvent;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.widget.Button;
import android.widget.EditText;
import android.widget.ListView;
import android.widget.TextView;

import java.util.ArrayList;
import java.util.concurrent.TimeUnit;

public class MainActivity extends Activity {
  final static String TAG = "InputOverlayTest";

  private Button mButton;
  private EditText mEdit;
  private ListView mList;
  private EventListAdapter mAdapter;
  private ArrayList<ReceivedEvent> mEvents;
  private View mView;

  @Override
  protected void onCreate(Bundle savedInstanceState) {
    super.onCreate(savedInstanceState);
    setContentView(R.layout.activity_main);

    mEdit = findViewById(R.id.m_edit);
    mButton = findViewById(R.id.m_button);
    mList = findViewById(R.id.m_list);
    mEvents = new ArrayList<>();
    mAdapter = new EventListAdapter(getApplicationContext(), mEvents);
    mList.setAdapter(mAdapter);
    mView = findViewById(R.id.main_view);
  }

  @Override
  public boolean dispatchTouchEvent(MotionEvent ev) {
    printEvent(ev);
    // Stop dispatching gamepad event.
    if (isGameEvent(ev)) {
      return true;
    }
    return super.dispatchTouchEvent(ev);
  }

  @Override
  public boolean dispatchKeyEvent(KeyEvent ev) {
    if (ev.getKeyCode() == KeyEvent.KEYCODE_TAB && ev.getAction() == KeyEvent.ACTION_DOWN) {
      mEdit.clearFocus();
      mEdit.setCursorVisible(false);
      return false;
    } else if (ev.getKeyCode() == KeyEvent.KEYCODE_TAB && ev.getAction() == KeyEvent.ACTION_UP) {
      return false;
    }
    printEvent(ev);
    // Stop dispatching gamepad event.
    if (isGameEvent(ev)) {
      return true;
    }
    return super.dispatchKeyEvent(ev);
  }

  @Override
  public boolean dispatchGenericMotionEvent(MotionEvent ev) {
    printEvent(ev);
    // Stop dispatching gamepad event.
    if (isGameEvent(ev)) {
      return true;
    }
    return super.dispatchGenericMotionEvent(ev);
  }

  @Override
  public boolean dispatchTrackballEvent(MotionEvent ev) {
    printEvent(ev);
    return super.dispatchTrackballEvent(ev);
  }

  private void printEvent(InputEvent event) {
    Log.v(TAG, event.toString());
    ReceivedEvent ev = new ReceivedEvent(event, SystemClock.elapsedRealtimeNanos());
    mEvents.add(ev);
    mAdapter.notifyDataSetChanged();
  }

  private boolean isGameEvent(InputEvent event) {
    final int source = event.getSource();
    if ((source & InputDevice.SOURCE_GAMEPAD) != 0
        || (source & InputDevice.SOURCE_JOYSTICK) != 0) {
      return true;
    }
    return false;
  }
}