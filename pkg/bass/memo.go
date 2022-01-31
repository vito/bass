package bass

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// Memos is where memoized calls are cached.
type Memos interface {
	Store(category Symbol, input Value, output Value) error
	Retrieve(category Symbol, input Value) (Value, bool, error)
	Remove(category Symbol, input Value) error
}

func init() {
	Ground.Set("memo",
		Func("memo", "[f category]", func(f Combiner, memos Path, category Symbol) Combiner {
			return Wrap(Op("memo", "[selector]", func(ctx context.Context, cont Cont, scope *Scope, input Value) ReadyCont {
				memo, err := OpenMemos(ctx, memos)
				if err != nil {
					return cont.Call(nil, fmt.Errorf("open memos: %w", err))
				}

				res, found, err := memo.Retrieve(category, input)
				if err != nil {
					return cont.Call(nil, fmt.Errorf("retrieve memo %s: %w", category, err))
				}

				if found {
					return cont.Call(res, nil)
				}

				return f.Call(ctx, NewList(input), scope, Continue(func(res Value) Value {
					err := memo.Store(category, input, res)
					if err != nil {
						return cont.Call(nil, fmt.Errorf("store memo %s: %w", category, err))
					}

					return cont.Call(res, nil)
				}))
			}))
		}))

	Ground.Set("unmemo",
		Func("unmemo", "[memos category filter]", func(ctx context.Context, memos Path, category Symbol, filter *Scope) error {
			memo, err := OpenMemos(ctx, memos)
			if err != nil {
				return fmt.Errorf("open memos: %w", err)
			}

			return memo.Remove(category, filter)
		}))
}

type Lockfile struct {
	path string
	lock *flock.Flock
}

type LockfileContent struct {
	Data Data `json:"memo"`
}

type Data map[Symbol][]Memory

type Memory struct {
	Input  ValueJSON `json:"input"`
	Output ValueJSON `json:"output"`
}

// ValueJSON is just an envelope for an arbitrary Value.
type ValueJSON struct {
	Value
}

func (res *ValueJSON) UnmarshalJSON(p []byte) error {
	var val interface{}
	err := UnmarshalJSON(p, &val)
	if err != nil {
		return err
	}

	value, err := ValueOf(val)
	if err != nil {
		return err
	}

	res.Value = value

	return nil
}

func (res ValueJSON) MarshalJSON() ([]byte, error) {
	return MarshalJSON(res.Value)
}

const LockfileName = "bass.lock"

func OpenMemos(ctx context.Context, dir Path) (Memos, error) {
	var hostPath HostPath
	if err := dir.Decode(&hostPath); err == nil {
		if hostPath.Path.FilesystemPath().IsDir() {
			if lf, ok := searchLockfile(hostPath.FromSlash()); ok {
				return lf, nil
			} else {
				return NoopMemos{}, nil
			}
		} else {
			return NewLockfileMemo(hostPath.FromSlash()), nil
		}
	}

	var thunkPath ThunkPath
	if err := dir.Decode(&thunkPath); err == nil {
		// TODO: read-only repository that locates via exporting and calling .Dir()
		// until it's found (TODO: cache heavily)
		return NoopMemos{}, nil
	}

	var fsPath FSPath
	if err := dir.Decode(&fsPath); err == nil {
		// NB: this is intentional; there aren't any pinned dependencies in stdlib.
		return NoopMemos{}, nil
	}

	return nil, fmt.Errorf("cannot locate memosphere in %T: %s", dir, dir)
}

func searchLockfile(startDir string) (*Lockfile, bool) {
	here := filepath.Join(startDir, LockfileName)
	if _, err := os.Stat(here); err == nil {
		return NewLockfileMemo(here), true
	}

	parent := filepath.Dir(startDir)
	if parent == startDir {
		// reached root
		return nil, false
	}

	return searchLockfile(parent)
}

func NewLockfileMemo(path string) *Lockfile {
	return &Lockfile{
		path: path,
		lock: flock.New(path),
	}
}

var _ Memos = &Lockfile{}

func (file *Lockfile) Store(category Symbol, input Value, output Value) error {
	err := file.lock.Lock()
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}

	defer file.lock.Unlock()

	content, err := file.load()
	if err != nil {
		return fmt.Errorf("load lock file: %w", err)
	}

	entries, found := content.Data[category]
	if !found {
		entries = []Memory{}
	}

	var updated bool
	for i, e := range entries {
		if e.Input.Equal(input) {
			entries[i].Output = ValueJSON{output}
			updated = true
		}
	}

	if !updated {
		entries = append(entries, Memory{ValueJSON{input}, ValueJSON{output}})
	}

	content.Data[category] = entries

	return file.save(content)
}

func (file *Lockfile) Retrieve(category Symbol, input Value) (Value, bool, error) {
	err := file.lock.RLock()
	if err != nil {
		return nil, false, fmt.Errorf("lock: %w", err)
	}

	defer file.lock.Unlock()

	content, err := file.load()
	if err != nil {
		return nil, false, fmt.Errorf("load lock file: %w", err)
	}

	entries, found := content.Data[category]
	if !found {
		return nil, false, nil
	}

	for _, e := range entries {
		if e.Input.Equal(input) {
			return e.Output, true, nil
		}
	}

	return nil, false, nil
}

func (file *Lockfile) Remove(category Symbol, input Value) error {
	err := file.lock.Lock()
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}

	defer file.lock.Unlock()

	content, err := file.load()
	if err != nil {
		return fmt.Errorf("load lock file: %w", err)
	}

	entries, found := content.Data[category]
	if !found {
		return nil
	}

	kept := []Memory{}
	for _, e := range entries {
		// TODO: would be nice to support IsSubsetOf semantics
		if !input.Equal(e.Input) {
			kept = append(kept, e)
		}
	}

	if len(kept) == 0 {
		delete(content.Data, category)
	} else {
		content.Data[category] = kept
	}

	return file.save(content)
}

func (file *Lockfile) load() (*LockfileContent, error) {
	payload, err := os.ReadFile(file.path)
	if err != nil {
		return nil, fmt.Errorf("read lock: %w", err)
	}

	content := LockfileContent{
		Data: Data{},
	}

	err = UnmarshalJSON(payload, &content)
	if err != nil {
		var syn *json.SyntaxError
		if errors.As(err, &syn) && syn.Error() == "unexpected end of JSON input" {
			return &content, nil
		}

		if errors.Is(err, io.EOF) {
			return &content, nil
		}

		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	for c, es := range content.Data {
		filtered := []Memory{}
		for _, e := range es {
			if e.Input.Value == nil || e.Output.Value == nil {
				// filter any corrupt entries
				continue
			}

			filtered = append(filtered, e)
		}

		if len(filtered) == 0 {
			delete(content.Data, c)
		} else {
			content.Data[c] = filtered
		}
	}

	return &content, nil
}

func (file *Lockfile) save(content *LockfileContent) error {
	buf := new(bytes.Buffer)
	enc := NewEncoder(buf)
	enc.SetIndent("", "  ")

	err := enc.Encode(content)
	if err != nil {
		return err
	}

	return os.WriteFile(file.path, buf.Bytes(), 0644)
}

type NoopMemos struct{}

var _ Memos = NoopMemos{}

func (NoopMemos) Store(Symbol, Value, Value) error {
	return nil
}

func (NoopMemos) Retrieve(Symbol, Value) (Value, bool, error) {
	return nil, false, nil
}

func (NoopMemos) Remove(Symbol, Value) error {
	return nil
}
