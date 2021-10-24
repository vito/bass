package ghcmd

import (
	"fmt"
	"regexp"
	"strings"
)

// Command is a workflow command parsed from output.
type Command struct {
	Name   string
	Params Params
	Value  string
}

func (cmd Command) String() string {
	if len(cmd.Params) > 0 {
		return fmt.Sprintf("::%s %s::%s", cmd.Name, cmd.Params, cmd.Value)
	} else {
		return fmt.Sprintf("::%s::%s", cmd.Name, cmd.Value)
	}
}

// Params is a set of named parameters for the command.
type Params map[string]string

func (params Params) String() string {
	ps := []string{}
	for k, v := range params {
		ps = append(ps, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(ps, ",")
}

var cmdRe = regexp.MustCompile(`^` + Dispatch + `([^\s]+)(\s+(.+))?` + Dispatch + `(.*)$`)

// ParseCommand parses the command from cmdline, which must not have a trailing
// linebreak.
func ParseCommand(cmdline string) (*Command, error) {
	res := cmdRe.FindStringSubmatch(cmdline)
	if res == nil {
		return nil, fmt.Errorf("malformed command: %q", cmdline)
	}

	cmd := &Command{
		Name:   res[1],
		Value:  res[4],
		Params: Params{},
	}

	for _, param := range strings.Split(res[3], ",") {
		if param == "" {
			continue
		}

		kv := strings.SplitN(param, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("malformed command kv: %q", kv)
		}

		cmd.Params[kv[0]] = kv[1]
	}

	return cmd, nil
}
