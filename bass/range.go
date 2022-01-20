package bass

import (
	"fmt"

	"github.com/spy16/slurp/reader"
)

type Range struct {
	Start, End reader.Position
}

func (inner Range) IsWithin(outer Range) bool {
	if inner.Start.Ln < outer.Start.Ln {
		return false
	}

	if inner.End.Ln > outer.End.Ln {
		return false
	}

	if inner.Start.Ln == outer.Start.Ln {
		if inner.Start.Col < outer.Start.Col {
			return false
		}
	}

	if inner.End.Ln == outer.End.Ln {
		if inner.End.Col > outer.End.Col {
			return false
		}
	}

	return true
}

func (r Range) String() string {
	return fmt.Sprintf("%s:%d:%d..%d:%d", r.Start.File, r.Start.Ln, r.Start.Col, r.End.Ln, r.End.Col)
}

func (r *Range) FromMeta(meta *Scope) error {
	if err := meta.GetDecode("file", &r.Start.File); err != nil {
		return err
	}

	if err := meta.GetDecode("line", &r.Start.Ln); err != nil {
		return err
	}

	if err := meta.GetDecode("column", &r.Start.Col); err != nil {
		return err
	}

	return nil
}

func (r Range) ToMeta(meta *Scope) {
	meta.Set("file", String(r.Start.File))
	meta.Set("line", Int(r.Start.Ln))
	meta.Set("column", Int(r.Start.Col))
}
