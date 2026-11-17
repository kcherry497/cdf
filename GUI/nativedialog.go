package main

// nativedialog.go — shells out to zenity for a genuine GTK file dialog
// (the one your file manager uses), instead of Fyne's built-in dialog
// which doesn't show mounted disks in the sidebar. Falls back to
// Fyne's dialog on non-Linux systems or when zenity isn't installed.
//
// Once zenity's window maps, xdotool swaps GTK's breadcrumb for a typeable,
// preselected path (Nemo's Ctrl+L feel). Best-effort: without X, xdotool,
// or a findable window it does nothing and the dialog behaves as before.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// hasZenity returns true when zenity is on PATH (Linux only).
func hasZenity() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("zenity")
	return err == nil
}

// nativeOpen shows a zenity file-selection dialog and returns the
// selected path, or ("", false) if user cancelled / zenity failed.
// Blocks until user closes the dialog.
func nativeOpen(startDir string) (string, bool) {
	if !hasZenity() { return "", false }
	const title = "Open CDF"
	args := []string{
		"--file-selection",
		"--title=" + title,
		"--file-filter=CDF documents | *.cdf *.json",
		"--file-filter=All files | *",
	}
	if startDir != "" {
		args = append(args, "--filename="+startDir+"/")
	}
	cmd := exec.Command("zenity", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil { return "", false }
	stop := startPrefill(cmd.Process.Pid, title, openSteps(startDir))
	err := cmd.Wait()
	stop()
	if err != nil { return "", false }   // cancel = non-zero exit
	return strings.TrimSpace(out.String()), true
}

// nativeSave shows a zenity save-as dialog and returns the path chosen.
func nativeSave(title, defaultName, startDir string) (string, bool) {
	if !hasZenity() { return "", false }
	if title == "" { title = "Save As" }
	args := []string{
		"--file-selection",
		"--save",
		"--confirm-overwrite",
		"--title=" + title,
	}
	if defaultName != "" {
		fn := defaultName
		if startDir != "" { fn = startDir + "/" + defaultName }
		args = append(args, "--filename="+fn)
	} else if startDir != "" {
		args = append(args, "--filename="+startDir+"/")
	}
	cmd := exec.Command("zenity", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil { return "", false }
	stop := startPrefill(cmd.Process.Pid, title, saveSteps(startDir, defaultName))
	err := cmd.Wait()
	stop()
	if err != nil { return "", false }
	return strings.TrimSpace(out.String()), true
}

// withSlash normalises a dir to exactly one trailing slash ("/" stays "/").
func withSlash(dir string) string {
	return strings.TrimRight(dir, "/") + "/"
}

// splitStem mirrors GTK's Name selection: the stem runs to the last dot,
// and a dotfile like ".bashrc" is all stem.
func splitStem(name string) (stem, ext string) {
	ext = filepath.Ext(name)
	if ext == name { return name, "" }
	return strings.TrimSuffix(name, ext), ext
}

// isASCII: xdotool types non-ASCII via a scratch keycode it remaps and restores
// at once; a busy file chooser reads the key after the restore and drops it.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf { return false }
	}
	return true
}

// openSteps: Ctrl+L turns the breadcrumb into an empty location entry; the
// folder is typed with a trailing / then selected so typing replaces it.
func openSteps(startDir string) [][]string {
	steps := [][]string{{"key", "--clearmodifiers", "ctrl+l"}}
	if startDir == "" || !isASCII(startDir) { return steps }   // never type a mangled path
	return append(steps,
		[]string{"type", "--clearmodifiers", "--delay", "1", "--", withSlash(startDir)},
		[]string{"key", "--clearmodifiers", "ctrl+a"},
	)
}

// saveSteps puts the full path in the Name entry, then selects just the stem.
// No startDir (or non-ASCII) leaves GTK's own Name + stem selection alone.
func saveSteps(startDir, defaultName string) [][]string {
	full := withSlash(startDir) + defaultName
	if startDir == "" || !isASCII(full) { return nil }
	steps := [][]string{
		{"key", "--clearmodifiers", "ctrl+a"},
		{"type", "--clearmodifiers", "--delay", "1", "--", full},
	}
	if defaultName == "" { return steps }
	stem, ext := splitStem(defaultName)
	selection := []string{"key", "--clearmodifiers", "--delay", "1", "End"}
	for i := 0; i < utf8.RuneCountInString(ext); i++ { selection = append(selection, "Left") }
	for i := 0; i < utf8.RuneCountInString(stem); i++ { selection = append(selection, "shift+Left") }
	return append(steps, selection)
}

// canDriveDialog: xdotool only reaches X windows (no DISPLAY = pure Wayland/none).
func canDriveDialog() bool {
	if os.Getenv("DISPLAY") == "" { return false }
	_, err := exec.LookPath("xdotool")
	return err == nil
}

// startPrefill runs steps against zenity's window in the background. Callers
// stop() right after cmd.Wait(); it cancels and waits, so nothing outlives the dialog.
func startPrefill(pid int, title string, steps [][]string) (stop func()) {
	if len(steps) == 0 || !canDriveDialog() { return func() {} }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		prefillDialog(ctx, pid, title, steps)
	}()
	return func() { cancel(); <-done }
}

// prefillDialog activates the dialog and sends steps, re-checking focus before
// each one so keys never land in another window.
func prefillDialog(ctx context.Context, pid int, title string, steps [][]string) {
	id := findDialogWindow(ctx, pid, title)
	if id == "" { return }
	// Prefill is best-effort; activation, focus, or input failures are deliberately silent.
	if _, err := dialogQuery(ctx, "windowactivate", "--sync", id); err != nil { return }
	if !sleepCtx(ctx, 150*time.Millisecond) { return }
	for _, step := range steps {
		active, err := dialogQuery(ctx, "getactivewindow")
		if err != nil || strings.TrimSpace(active) != id { return }
		if ctx.Err() != nil { return }
		// Let each input finish releasing keys even if the dialog exits meanwhile.
		inputCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(inputCtx, "xdotool", step...)
		cmd.WaitDelay = 500 * time.Millisecond
		err = cmd.Run()
		cancel()
		if err != nil { return } // Prefill errors must not change the dialog result.
	}
}

// dialogQuery runs a read-only xdotool call; killing it on cancel is safe.
func dialogQuery(ctx context.Context, args ...string) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(queryCtx, "xdotool", args...)
	cmd.WaitDelay = 500 * time.Millisecond
	out, err := cmd.Output()
	return string(out), err
}

// parseWindowIDs keeps only all-digit lines from xdotool search output.
func parseWindowIDs(out string) []string {
	var ids []string
	for _, line := range strings.Split(out, "\n") {
		id := strings.TrimSpace(line)
		if id == "" { continue }
		valid := true
		for _, digit := range id {
			if digit < '0' || digit > '9' { valid = false; break }
		}
		if valid { ids = append(ids, id) }
	}
	return ids
}

// searchDialogWindows lists visible windows matching the search args.
func searchDialogWindows(ctx context.Context, args ...string) []string {
	out, err := dialogQuery(ctx, append([]string{"search", "--onlyvisible"}, args...)...)
	if err != nil { return nil } // No matches (exit 1) or query failure: prefill is best-effort.
	return parseWindowIDs(out)
}

// findDialogWindow polls (100 ms, up to 5 s) for zenity's window by pid, falling
// back to a single exact-title match that wasn't already on screen at start.
func findDialogWindow(ctx context.Context, pid int, title string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	pattern := "^" + regexp.QuoteMeta(title) + "$"
	preexisting := make(map[string]bool)
	for _, id := range searchDialogWindows(ctx, "--name", pattern) { preexisting[id] = true }
	for ctx.Err() == nil {
		ids := searchDialogWindows(ctx, "--pid", strconv.Itoa(pid))
		if ctx.Err() != nil { return "" }
		if len(ids) == 1 { return ids[0] }
		if len(ids) > 1 {
			for _, id := range ids {
				name, err := dialogQuery(ctx, "getwindowname", id)
				if err != nil { continue } // Best-effort title matching; keep the PID fallback.
				if strings.TrimSuffix(name, "\n") == title { return id }
			}
			if ctx.Err() != nil { return "" }
			return ids[0]
		}
		var candidates []string
		for _, id := range searchDialogWindows(ctx, "--name", pattern) {
			if !preexisting[id] { candidates = append(candidates, id) }
		}
		if ctx.Err() != nil { return "" }
		if len(candidates) == 1 { return candidates[0] }
		if !sleepCtx(ctx, 100*time.Millisecond) { return "" }
	}
	return ""
}

// sleepCtx sleeps for delay; false if ctx ended first.
func sleepCtx(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}
