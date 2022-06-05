package proto

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
)

func (thunk *Thunk) SHA256() (string, error) {
	payload, err := proto.Marshal(thunk)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)
	return base64.URLEncoding.EncodeToString(sum[:]), nil
}

// Cmdline returns a human-readable representation of the thunk's command and
// args.
func (thunk *Thunk) Cmdline() string {
	var cmdline []string

	switch x := thunk.Cmd.Cmd.(type) {
	case *ThunkCmd_CommandCmd:
		cmdline = append(cmdline, x.CommandCmd.Command)
	case *ThunkCmd_FileCmd:
		cmdline = append(cmdline, x.FileCmd.Path)
	case *ThunkCmd_FsCmd:
		cmdline = append(cmdline, fmt.Sprintf("<fs %s>/%s", x.FsCmd.Id, x.FsCmd.Path.Slash()))
	case *ThunkCmd_HostCmd:
		cmdline = append(cmdline, fmt.Sprintf("<host %s>/%s", x.HostCmd.Context, x.HostCmd.Path.Slash()))
	case *ThunkCmd_ThunkCmd:
		digest, err := x.ThunkCmd.Thunk.SHA256()
		if err != nil {
			panic(err)
		}

		cmdline = append(cmdline, fmt.Sprintf("<thunk %s>/%s", digest, x.ThunkCmd.Path.Slash()))
	default:
		panic(fmt.Sprintf("unknown command type: %T", x))
	}

	for _, arg := range thunk.Args {
		str := arg.GetStringValue()
		if str != nil {
			cmdline = append(cmdline, str.GetInner())
		} else {
			// TODO
			cmdline = append(cmdline, arg.String())
		}
	}

	return strings.Join(cmdline, " ")
}
