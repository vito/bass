package lsp_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/neovim/go-client/nvim"
	"github.com/stretchr/testify/require"
)

func TestNeovimGoToDefinition(t *testing.T) {
	testFile(t, sandboxNvim(t), "testdata/gd.bass")
}

func TestNeovimCompletion(t *testing.T) {
	testFile(t, sandboxNvim(t), "testdata/complete.bass")
}

func testFile(t *testing.T, client *nvim.Nvim, file string) {
	err := client.Command(`edit ` + file)
	require.NoError(t, err)

	testBuf, err := client.CurrentBuffer()
	require.NoError(t, err)

	window, err := client.CurrentWindow()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		var b bool
		err := client.Eval(`luaeval('#vim.lsp.buf_get_clients() > 0')`, &b)
		return err == nil && b
	}, time.Second, 10*time.Millisecond, "LSP client did not attach")

	lineCount, err := client.BufferLineCount(testBuf)
	require.NoError(t, err)

	t.Logf("lines: %d", lineCount)

	for testLine := 1; testLine <= lineCount; testLine++ {
		mode, err := client.Mode()
		require.NoError(t, err)

		if mode.Mode != "n" {
			// reset back to normal mode; some tests can't <esc> immediately because
			// they have to wait for the language server (e.g. completion)
			err = client.FeedKeys("\x1b", "t", true)
			require.NoError(t, err)
		}

		err = client.SetWindowCursor(window, [2]int{testLine, 0})
		require.NoError(t, err)

		lineb, err := client.CurrentLine()
		require.NoError(t, err)
		line := string(lineb)

		segs := strings.Split(line, "; test: ")
		if len(segs) < 2 {
			continue
		}

		eq := strings.Split(segs[1], " => ")

		codes := strings.TrimSpace(eq[0])
		keys, err := client.ReplaceTermcodes(codes, true, true, true)
		require.NoError(t, err)

		err = client.FeedKeys(keys, "t", true)
		require.NoError(t, err)

		targetPos := strings.Index(eq[1], "^")
		target := strings.ReplaceAll(eq[1], "^", "")
		target = strings.ReplaceAll(target, "\\t", "\t")

		// wait for the definition to be found
		require.Eventually(t, func() bool {
			line, err := client.CurrentLine()
			require.NoError(t, err)

			pos, err := client.WindowCursor(window)
			require.NoError(t, err)

			idx := strings.Index(string(line), target)
			if idx == -1 {
				t.Logf("L%03d %s\tline %q does not contain %q", testLine, codes, string(line), target)
				return false
			}

			col := targetPos + idx // account for leading whitespace

			if pos[1] != col {
				t.Logf("L%03d %s\tline %q: at %d, need %d", testLine, codes, string(line), col, pos[1])
				return false
			}

			t.Logf("L%03d %s\tmatched: %s", testLine, codes, eq[1])

			return true
		}, 1*time.Second, 10*time.Millisecond)

		// go back from definition to initial test buffer
		err = client.SetCurrentBuffer(testBuf)
		require.NoError(t, err)
	}
}

func sandboxNvim(t *testing.T) *nvim.Nvim {
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
		nvim.ChildProcessArgs("--clean", "-n", "--embed", "--headless", "--noplugin"),
		nvim.ChildProcessContext(ctx),
		nvim.ChildProcessLogf(t.Logf),
	)
	require.NoError(t, err)

	err = client.Command(`source testdata/config.vim`)
	require.NoError(t, err)

	paths, err := client.RuntimePaths()
	require.NoError(t, err)

	t.Logf("runtimepath: %v", paths)

	return client
}
