# Local sandbox UI integration test

This page is the instruction pane for the local sandbox UI integration test. The terminal pane is a real shell connected over SSH. Run commands only in that terminal unless a step explicitly asks you to use the host clipboard or a host text editor.

The checks that inspect the operating-system clipboard or the browser's native context menu are marked **Human-only browser check**. Headless browser automation does not establish those results.

## 1. Confirm the connected shell

- **Action:** In the terminal pane, run:

  ```sh
  printf '%s\n' 'P3_SHELL_CONNECTED_9K4'
  ```

- **Unique payload:** `P3_SHELL_CONNECTED_9K4`
- **Expected result:** The terminal prints one line containing `P3_SHELL_CONNECTED_9K4` and returns to a usable shell prompt.
- **Failure condition:** The marker is absent, altered, or printed by instructions rather than command output, or the terminal does not return to a usable prompt.

## 2. Drag to copy through tmux

**Human-only browser check:** this scenario verifies the host operating-system clipboard.

- **Action:** Run:

  ```sh
  printf '%s\n' 'P3_TMUX_DRAG_COPY_V6N'
  ```

  Without holding a modifier key, drag from the first `P` through the final `N` in the printed marker. Release inside the terminal pane. Paste the host clipboard into a temporary plain-text editor outside the sandbox UI; do not paste it into the shell.

- **Unique payload:** `P3_TMUX_DRAG_COPY_V6N`
- **Expected result:** tmux selects the terminal text, and the host paste contains `P3_TMUX_DRAG_COPY_V6N` from the completed drag. A browser clipboard-permission prompt may require approval.
- **Failure condition:** After any required permission is granted, the clipboard still contains stale text, contains a different payload, or cannot receive the selected marker from a drag completed inside the terminal.

## 3. Use native terminal selection

**Human-only browser check:** this scenario verifies native selection and the host operating-system clipboard.

- **Action:** Run:

  ```sh
  printf '%s\n' 'P3_NATIVE_SELECT_R3C'
  ```

  Hold **Shift** on Linux or Windows, or **Option** on macOS, while dragging from the first `P` through the final `C`. Release inside the terminal pane. Paste into a temporary host plain-text editor. If the browser does not permit automatic clipboard writes, keep the selection active and use **Ctrl+Shift+C** on Linux or Windows, **Cmd+C** on macOS, or the browser's Copy command.

- **Unique payload:** `P3_NATIVE_SELECT_R3C`
- **Expected result:** The modifier drag creates an xterm-native selection instead of a tmux mouse selection. With clipboard permission, the host paste contains `P3_NATIVE_SELECT_R3C`; otherwise, the stated keyboard or browser copy fallback produces that payload.
- **Failure condition:** The modifier drag is handled as ordinary tmux selection, or neither the direct copy nor the explicit fallback can place `P3_NATIVE_SELECT_R3C` on the host clipboard.

## 4. Enter and move within tmux copy mode with the wheel

- **Action:** Run this command to print enough uniquely labeled lines to exceed the viewport:

  ```sh
  i=1; while [ "$i" -le 80 ]; do printf 'P3_WHEEL_COPY_H5D_%03d\n' "$i"; i=$((i + 1)); done
  ```

  Place the pointer over the terminal and wheel upward several times, then wheel down without clicking or holding a modifier.

- **Unique payload:** lines `P3_WHEEL_COPY_H5D_001` through `P3_WHEEL_COPY_H5D_080`
- **Expected result:** Wheel-up enters tmux copy mode and moves the viewport toward earlier numbered lines; wheel-down moves toward later lines. The shell must not receive literal wheel escape text as a command.
- **Failure condition:** The viewport does not move through the numbered output, scrolling leaves the terminal unusable, or wheel input appears at the shell prompt as typed data.

## 5. Preserve unmodified browser right-click

**Human-only browser check:** native context-menu rendering cannot be validated by headless browser automation.

- **Action:** First return to the live prompt if needed, then run:

  ```sh
  printf '%s\n' 'P3_RIGHT_CLICK_J8W'
  ```

  Right-click once over the terminal without holding Shift, Option, Ctrl, Alt, or Command. Dismiss the menu without choosing an item.

- **Unique payload:** `P3_RIGHT_CLICK_J8W`
- **Expected result:** The browser's native context menu opens. No tmux right-click menu opens, the marker is not pasted or executed, and the terminal remains usable after the menu is dismissed.
- **Failure condition:** The browser menu is suppressed, a tmux menu appears, right-click inserts or executes text, or dismissing the menu leaves the terminal unusable.

## 6. Preserve platform-native middle-click behavior

**Human-only browser check:** middle-button behavior belongs to the browser and operating system and cannot be validated by headless browser automation.

- **Action on Linux with primary-selection paste:** Select this complete command, including its final semicolon, without explicitly copying it to the system clipboard:

  ```sh
  printf '%s\n' 'P3_MIDDLE_NATIVE_7Q2';
  ```

  At an empty shell prompt, middle-click exactly once inside the terminal. Do not click again. Then press **Enter** separately once.
- **Action on macOS or platforms without primary-selection paste:** Middle-click once over an empty terminal prompt and confirm that the terminal follows the platform's normal behavior. Then use keyboard or browser-context-menu Paste to confirm that portable paste remains usable.
- **Unique payload:** `P3_MIDDLE_NATIVE_7Q2`
- **Expected result:** On Linux environments that provide primary-selection paste, one middle-click pastes one copy of the selected command and pressing Enter prints exactly one marker line. Platforms without that convention are not expected to paste on middle-click. The terminal bridge does not read the system clipboard, synthesize a paste, or suppress the browser's middle-button behavior.
- **Failure condition:** One middle-click is suppressed, produces two insertions, combines primary-selection and system-clipboard contents, or leaves the terminal unusable. Lack of middle-click paste on a platform without primary-selection semantics is not a failure.

## 7. Reject unsolicited OSC 52 clipboard output

**Human-only browser check:** this scenario verifies that the host operating-system clipboard is unchanged.

- **Action:** At an empty terminal prompt, enter the following command **without pressing Enter**. You may type it manually, or copy it from the instruction pane and use keyboard/browser Paste; do not use a terminal drag.

  ```sh
  printf '\033]52;c;UDNfT1NDX1VOU09MSUNJVEVEX0JMT0NLXzRNOA==\007'
  ```

  After the complete unexecuted command is visible at the prompt, copy only this sentinel from the instruction pane to the host clipboard:

  `P3_OSC_SENTINEL_2L6`

  Do not drag or select terminal text. Refocus the terminal and press **Enter** once, then paste the host clipboard into a temporary plain-text editor outside the sandbox UI.

- **Unique payloads:** clipboard sentinel `P3_OSC_SENTINEL_2L6`; rejected write payload `P3_OSC_UNSOLICITED_BLOCK_4M8`
- **Expected result:** The terminal may show no visible output from the control sequence. The host clipboard remains `P3_OSC_SENTINEL_2L6` and does not become `P3_OSC_UNSOLICITED_BLOCK_4M8`, because no qualifying completed terminal drag authorized an OSC 52 write.
- **Failure condition:** The host clipboard becomes `P3_OSC_UNSOLICITED_BLOCK_4M8` or otherwise changes as a result of this unsolicited sequence.

## 8. Observe the cross-iframe release limitation

**Human-only browser check:** pointer release and clipboard behavior across document boundaries depend on the browser and operating system.

- **Action:** Run:

  ```sh
  printf '%s\n' 'P3_IFRAME_RELEASE_X9F'
  ```

  Start a native selection in the terminal by holding **Shift** on Linux or Windows, or **Option** on macOS. Drag toward the instruction pane and release the pointer over this instruction iframe rather than inside the terminal iframe. If that boundary release does not complete or copy the selection, repeat the modifier drag entirely inside the terminal and use **Ctrl+Shift+C** on Linux or Windows, **Cmd+C** on macOS, or the browser's Copy command. Verify the fallback in a temporary host plain-text editor.

- **Unique payload:** `P3_IFRAME_RELEASE_X9F`
- **Expected result:** Release or copy across the iframe boundary may fail because the terminal document may not receive the release event; successful boundary copy is not required. The contained selection plus keyboard/browser fallback should copy `P3_IFRAME_RELEASE_X9F` and leave the terminal usable.
- **Failure condition:** Treating a missed cross-iframe release as a guaranteed-copy failure is incorrect. The actionable failure is that, after repeating the selection wholly inside the terminal, the keyboard/browser fallback cannot copy `P3_IFRAME_RELEASE_X9F` or the terminal remains stuck or unusable.
