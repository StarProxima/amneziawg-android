package crashoutput

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const helperEnv = "AWG_CRASH_OUTPUT_HELPER"

func TestCrashOutputHelper(t *testing.T) {
	kind := os.Getenv(helperEnv)
	if kind == "" {
		t.Skip("runs only as a crashing child process")
	}
	if err := Set(os.Getenv("AWG_CRASH_OUTPUT_PATH")); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		switch kind {
		case "panic":
			crashWithPanic()
		case "fatal":
			var mu sync.Mutex
			mu.Unlock()
		}
	}()
	<-done
}

func crashWithPanic() {
	var peers map[string]chan int
	close(peers["missing"])
}

func runCrashingChild(t *testing.T, kind string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "go-crash.txt")
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashOutputHelper$")
	// The same traceback level the runtime forces in an Android app process.
	cmd.Env = append(os.Environ(), helperEnv+"="+kind, "AWG_CRASH_OUTPUT_PATH="+path, "GOTRACEBACK=none")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("child did not crash:\n%s", output)
	}
	report, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(report)
}

func TestPanicIsRecordedWithFailingFrame(t *testing.T) {
	report := runCrashingChild(t, "panic")
	if !strings.HasPrefix(report, "panic: close of nil channel") {
		t.Fatalf("panic reason missing:\n%s", report)
	}
	if !strings.Contains(report, ".crashWithPanic(") || !strings.Contains(report, "crashoutput_test.go:") {
		t.Fatalf("failing frame missing:\n%s", report)
	}
}

// The runtime prints the reason of its own fatal errors before it switches
// into crash mode, so only the goroutine dump reaches the file
// (golang/go#76859). Consumers recover the kind from the stack.
func TestRuntimeFatalIsRecordedWithoutReason(t *testing.T) {
	report := runCrashingChild(t, "fatal")
	if strings.Contains(report, "fatal error:") {
		t.Fatalf("runtime now writes the fatal reason, update the report parsing:\n%s", report)
	}
	if !strings.Contains(report, "goroutine ") || !strings.Contains(report, "sync.fatal") {
		t.Fatalf("fatal stack missing:\n%s", report)
	}
}

func TestEmptyPathIsRejected(t *testing.T) {
	if err := Set(""); err == nil {
		t.Fatal("empty path accepted")
	}
}
