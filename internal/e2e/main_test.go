// ABOUTME: Builds the chronicle binary once and runs it as a real process for the end-to-end tests.
// ABOUTME: Each process gets a temp HOME, XDG_DATA_HOME and XDG_CONFIG_HOME, so none can touch real data.

//go:build !windows

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// runTimeout bounds a chronicle process that runs to completion, such as one add.
	runTimeout = 20 * time.Second
	// stopTimeout is how long cleanup waits for an interrupted process before it kills it.
	stopTimeout = 5 * time.Second
)

// chronicleBinary is the path of the binary that TestMain builds.
var chronicleBinary string

// processes maps every command that startChronicle started to its process.
var processes sync.Map

// process is a running chronicle process. One goroutine waits for it, so a test and cleanup can
// both ask whether it has exited without calling Wait twice.
type process struct {
	cmd  *exec.Cmd
	done chan struct{} // Closed once the process has exited and been reaped.
}

func TestMain(m *testing.M) {
	os.Exit(buildAndRun(m))
}

// buildAndRun builds the binary into a temp folder, runs the tests and removes the folder. TestMain
// can't defer the removal itself, because os.Exit skips deferred calls.
func buildAndRun(m *testing.M) int {
	buildDir, err := os.MkdirTemp("", "chronicle-e2e-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: create build folder: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(buildDir) }()

	chronicleBinary = filepath.Join(buildDir, "chronicle")
	build := exec.Command("go", "build", "-o", chronicleBinary, "github.com/harper/chronicle")
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: build chronicle: %v\n%s", err, output)
		return 1
	}
	return m.Run()
}

// newEnv returns the environment for chronicle processes that use the given storage backend: a
// temp HOME, XDG_DATA_HOME and XDG_CONFIG_HOME, and a config file that selects the backend. It
// creates no data directory, so a test starts with the empty state of a first run.
func newEnv(t *testing.T, backend string) []string {
	t.Helper()
	home, dataHome, configHome := t.TempDir(), t.TempDir(), t.TempDir()

	configDir := filepath.Join(configHome, "chronicle")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatalf("create config folder: %v", err)
	}
	config := fmt.Sprintf(`{"backend":%q}`, backend)
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return []string{"HOME=" + home, "XDG_DATA_HOME=" + dataHome, "XDG_CONFIG_HOME=" + configHome}
}

// workDir returns the folder a process with this env runs in: its HOME. chronicle add looks for a
// .chronicle project file from the working directory up to HOME. Run from the repository, it could
// find one in a folder above the repository and write project logs there.
func workDir(env []string) string {
	for _, pair := range env {
		if home, found := strings.CutPrefix(pair, "HOME="); found {
			return home
		}
	}
	return ""
}

// runChronicle runs the binary to completion and returns what it wrote to stdout. A non-zero exit
// fails the test.
func runChronicle(t *testing.T, env []string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, chronicleBinary, args...)
	cmd.Env = env
	cmd.Dir = workDir(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("chronicle %s: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

// startChronicle starts the binary and returns the running command and a reader over its stdout.
// Cleanup sends the process os.Interrupt and waits for it, and kills a process that ignores the
// interrupt, so no test leaves one behind even when it fails. A test that wants the exit status
// sends the interrupt itself and calls waitForExit.
func startChronicle(t *testing.T, env []string, args ...string) (*exec.Cmd, *bufio.Reader) {
	t.Helper()
	// An os.Pipe in place of cmd.StdoutPipe, because Wait closes a StdoutPipe the moment the
	// process exits, and a test may not have read its last lines by then.
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	cmd := exec.Command(chronicleBinary, args...)
	cmd.Env = env
	cmd.Dir = workDir(env)
	cmd.Stdout = stdoutWrite
	cmd.Stderr = testLog{t}
	err = cmd.Start()
	_ = stdoutWrite.Close() // The child has its own copy.
	if err != nil {
		_ = stdoutRead.Close()
		t.Fatalf("start chronicle %s: %v", strings.Join(args, " "), err)
	}

	p := &process{cmd: cmd, done: make(chan struct{})}
	processes.Store(cmd, p)
	go func() {
		_ = cmd.Wait() // The exit status is in cmd.ProcessState.
		close(p.done)
	}()
	t.Cleanup(func() {
		p.stop(t)
		_ = stdoutRead.Close()
	})
	return cmd, bufio.NewReader(stdoutRead)
}

// stop interrupts the process if it still runs and waits until it has exited, killing it when it
// does not exit within stopTimeout.
func (p *process) stop(t *testing.T) {
	t.Helper()
	select {
	case <-p.done:
		return
	default:
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	select {
	case <-p.done:
	case <-time.After(stopTimeout):
		_ = p.cmd.Process.Kill()
		<-p.done
		t.Errorf("chronicle %s ignored an interrupt for %s, so cleanup killed it", strings.Join(p.cmd.Args[1:], " "), stopTimeout)
	}
}

// waitForExit waits until a process that startChronicle started has exited and returns its exit
// code, which is -1 for a process that a signal ended. It fails the test when the process still
// runs after the given time.
func waitForExit(t *testing.T, cmd *exec.Cmd, within time.Duration) int {
	t.Helper()
	value, _ := processes.Load(cmd)
	p, ok := value.(*process)
	if !ok {
		t.Fatal("waitForExit: the command was not started by startChronicle")
	}
	select {
	case <-p.done:
		return cmd.ProcessState.ExitCode()
	case <-time.After(within):
		t.Fatalf("chronicle %s was still running after %s", strings.Join(cmd.Args[1:], " "), within)
		return 0
	}
}

// readLine returns the next line from r without its newline. It fails the test when a whole line
// does not arrive within the given time or the stream ends first. After a timeout the read goes on
// in the background until the process ends, which cleanup makes happen.
func readLine(t *testing.T, r *bufio.Reader, within time.Duration) string {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	results := make(chan result, 1)
	go func() {
		line, err := r.ReadString('\n')
		results <- result{line, err}
	}()
	select {
	case res := <-results:
		if res.err != nil {
			t.Fatalf("read a line: %v (read %q)", res.err, res.line)
		}
		return strings.TrimRight(res.line, "\r\n")
	case <-time.After(within):
		t.Fatalf("no line arrived within %s", within)
		return ""
	}
}

// testLog writes a process's stderr into the test log, where a failing test shows it. The test
// logs from cleanup too, which waits for the process, so no write comes after the test ends.
type testLog struct{ t *testing.T }

func (l testLog) Write(p []byte) (int, error) {
	l.t.Logf("chronicle stderr: %s", bytes.TrimRight(p, "\n"))
	return len(p), nil
}
