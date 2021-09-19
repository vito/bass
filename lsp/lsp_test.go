package lsp_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neovim/go-client/nvim"
	"github.com/stretchr/testify/require"
)

func TestNeovim(t *testing.T) {
	client := sandboxNvim(t)

	err := client.Command(`edit testdata/test.bass`)
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

	for i := 1; i <= lineCount; i++ {
		err := client.SetWindowCursor(window, [2]int{i, 0})
		require.NoError(t, err)

		lineb, err := client.CurrentLine()
		require.NoError(t, err)
		line := string(lineb)

		segs := strings.Split(line, "; test: ")
		if len(segs) < 2 {
			continue
		}

		eq := strings.Split(segs[1], " => ")

		err = client.FeedKeys(strings.TrimSpace(eq[0]), "", false)
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
				t.Logf("line %q does not contain %q", string(line), target)
				return false
			}

			col := targetPos + idx // account for leading whitespace

			if pos[1] != col {
				t.Logf("%s: col %d != %d", eq[1], col, pos[1])
				return false
			}

			t.Logf("found definition: %s", eq[1])

			return true
		}, time.Second, 10*time.Millisecond)

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
		nvim.ChildProcessArgs("-u", "NONE", "-n", "--embed", "--headless", "--noplugin"),
		nvim.ChildProcessContext(ctx),
		nvim.ChildProcessLogf(t.Logf),
	)
	require.NoError(t, err)

	paths, err := client.RuntimePaths()
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	bundledPaths, err := filepath.Glob("./testdata/bundle/*")
	require.NoError(t, err)

	runtimePath := bundledPaths
	for _, path := range paths {
		if strings.HasPrefix(path, home+string(os.PathSeparator)+".") {
			// ignore user's dotfiles
			continue
		}

		runtimePath = append(runtimePath, path)
	}

	t.Logf("runtimepath: %v", runtimePath)

	err = client.Command(`set runtimepath=` + strings.Join(runtimePath, ","))
	require.NoError(t, err)

	err = client.Command(`source testdata/config.vim`)
	require.NoError(t, err)

	return client
}
