package lsp_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/neovim/go-client/nvim"
	"github.com/vito/is"
)

func TestNeovimGoToDefinition(t *testing.T) {
	testFile(t, sandboxNvim(t), "testdata/gd.bass")
}

func TestNeovimCompletion(t *testing.T) {
	testFile(t, sandboxNvim(t), "testdata/complete.bass")
}

func testFile(t *testing.T, client *nvim.Nvim, file string) {
	is := is.New(t)

	err := client.Command(`edit ` + file)
	is.NoErr(err)

	testBuf, err := client.CurrentBuffer()
	is.NoErr(err)

	window, err := client.CurrentWindow()
	is.NoErr(err)

	is.Eventually(func() bool { // wait for LSP client to attach
		var b bool
		err := client.Eval(`luaeval('#vim.lsp.buf_get_clients() > 0')`, &b)
		return err == nil && b
	}, time.Second, 10*time.Millisecond)

	lineCount, err := client.BufferLineCount(testBuf)
	is.NoErr(err)

	t.Logf("lines: %d", lineCount)

	t.Cleanup(func() {
		if !t.Failed() {
			return
		}

		lspLogs, err := os.ReadFile("bass-lsp.log")
		if err == nil {
			t.Logf("language server logs:\n\n%s", string(lspLogs))
		}
	})

	for testLine := 1; testLine <= lineCount; testLine++ {
		mode, err := client.Mode()
		is.NoErr(err)

		if mode.Mode != "n" {
			// reset back to normal mode; some tests can't <esc> immediately because
			// they have to wait for the language server (e.g. completion)
			err = client.FeedKeys("\x1b", "t", true)
			is.NoErr(err)
		}

		err = client.SetWindowCursor(window, [2]int{testLine, 0})
		is.NoErr(err)

		lineb, err := client.CurrentLine()
		is.NoErr(err)
		line := string(lineb)

		segs := strings.Split(line, "; test: ")
		if len(segs) < 2 {
			continue
		}

		eq := strings.Split(segs[1], " => ")

		codes := strings.TrimSpace(eq[0])
		keys, err := client.ReplaceTermcodes(codes, true, true, true)
		is.NoErr(err)

		err = client.FeedKeys(keys, "t", true)
		is.NoErr(err)

		targetPos := strings.Index(eq[1], "┃")
		target := strings.ReplaceAll(eq[1], "┃", "")
		target = strings.ReplaceAll(target, "\\t", "\t")

		is.Eventually(func() bool { // wait for the definition to be found
			line, err := client.CurrentLine()
			is.NoErr(err)

			pos, err := client.WindowCursor(window)
			is.NoErr(err)

			idx := strings.Index(string(line), target)
			if idx == -1 {
				t.Logf("L%03d %s\tline %q does not contain %q", testLine, codes, string(line), target)
				return false
			}

			col := targetPos + idx // account for leading whitespace

			if pos[1] != col {
				t.Logf("L%03d %s\tline %q: at %d, need %d", testLine, codes, string(line), pos[1], col)
				return false
			}

			t.Logf("L%03d %s\tmatched: %s", testLine, codes, eq[1])

			return true
		}, 1*time.Second, 10*time.Millisecond)

		// go back from definition to initial test buffer
		err = client.SetCurrentBuffer(testBuf)
		is.NoErr(err)
	}
}

func sandboxNvim(t *testing.T) *nvim.Nvim {
	is := is.New(t)

	ctx := context.Background()

	cmd := os.Getenv("BASS_LSP_NEOVIM_BIN")
	if cmd == "" {
		var err error
		cmd, err = exec.LookPath("nvim")
		if err != nil {
			t.Skip("nvim not installed; skipping LSP tests")
		}
	}

	client, err := nvim.NewChildProcess(
		nvim.ChildProcessCommand(cmd),
		nvim.ChildProcessArgs("--clean", "-n", "--embed", "--headless", "--noplugin", "-V10nvim.log"),
		nvim.ChildProcessContext(ctx),
		nvim.ChildProcessLogf(t.Logf),
	)
	is.NoErr(err)

	t.Cleanup(func() {
		err := client.Close()
		if err != nil {
			t.Logf("failed to close neovim: %s", err)
		}

		if t.Failed() {
			nvimLogs, err := os.ReadFile("nvim.log")
			if err == nil {
				t.Logf("neovim logs:\n\n%s", string(nvimLogs))
			}
		}
	})

	err = client.Command(`source testdata/config.vim`)
	is.NoErr(err)

	paths, err := client.RuntimePaths()
	is.NoErr(err)

	t.Logf("runtimepath: %v", paths)

	return client
}
