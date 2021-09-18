// +build !go1.13

package lsp

func succeeded(err error) bool {
	return err == nil
}
