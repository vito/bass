package bass

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/gofrs/flock"
)

// Memos is where memoized calls are cached.
type Memos interface {
	Store(Thunk, Symbol, Value, Value) error
	Retrieve(Thunk, Symbol, Value) (Value, bool, error)
	Remove(Thunk, Symbol, Value) error
}

func init() {
	Ground.Set("recall-memo",
		Func("recall-memo", "[memos thunk binding input]", func(ctx context.Context, memos Path, thunk Thunk, binding Symbol, input Value) (Value, error) {
			memo, err := OpenMemos(ctx, memos)
			if err != nil {
				return nil, fmt.Errorf("open memos at %s: %w", memos, err)
			}

			res, found, err := memo.Retrieve(thunk, binding, input)
			if err != nil {
				return nil, fmt.Errorf("retrieve memo %s:%s: %w", thunk, binding, err)
			}

			if found {
				return res, nil
			}

			return Null{}, nil
		}),
		`fetches the result of a memoized function call`,
		`Returns null if no result is found.`,
		`See (memo) for the higher-level interface.`)

	Ground.Set("store-memo",
		Func("store-memo", "[memos thunk binding input result]", func(ctx context.Context, memos Path, thunk Thunk, binding Symbol, input Value, res Value) (Value, error) {
			memo, err := OpenMemos(ctx, memos)
			if err != nil {
				return nil, fmt.Errorf("open memos at %s: %w", memos, err)
			}

			err = memo.Store(thunk, binding, input, res)
			if err != nil {
				return nil, fmt.Errorf("store memo %s:%s: %w", thunk, binding, err)
			}

			return res, nil
		}),
		`stores the result of a memoized function call`,
		`See (memo) for the higher-level interface.`)
}

type Lockfile struct {
	path string
	lock *flock.Flock
}

type LockfileContent struct {
	Data   MemoData         `json:"memo"`
	Thunks map[string]Thunk `json:"thunks"`
}

type MemoData map[string][]Memory

type Memory struct {
	Input  ValueJSON `json:"input"`
	Output ValueJSON `json:"output"`
}

// ValueJSON is just an envelope for an arbitrary Value.
type ValueJSON struct {
	Value
}

func (res *ValueJSON) UnmarshalJSON(p []byte) error {
	return UnmarshalJSON(p, &res.Value)
}

func (res ValueJSON) MarshalJSON() ([]byte, error) {
	return MarshalJSON(res.Value)
}

func OpenMemos(ctx context.Context, dir Path) (Memos, error) {
	var hostPath HostPath
	if err := dir.Decode(&hostPath); err == nil {
		return OpenHostPathMemos(hostPath), nil
	}

	var fsPath FSPath
	if err := dir.Decode(&fsPath); err == nil {
		return OpenFSPathMemos(fsPath)
	}

	var thunkPath ThunkPath
	if err := dir.Decode(&thunkPath); err == nil {
		return OpenThunkPathMemos(ctx, thunkPath)
	}

	return nil, fmt.Errorf("cannot locate memosphere in %T: %s", dir, dir)
}

func OpenHostPathMemos(hostPath HostPath) Memos {
	return NewLockfileMemo(hostPath.FromSlash())
}

func OpenFSPathMemos(fsPath FSPath) (Memos, error) {
	file, err := fsPath.FS.Open(fsPath.Path.File.Path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	dec := json.NewDecoder(file)

	var content LockfileContent
	err = dec.Decode(&content)
	if err != nil {
		return nil, err
	}

	return ReadonlyMemos{content}, nil
}

func OpenThunkPathMemos(ctx context.Context, thunkPath ThunkPath) (Memos, error) {
	cacheLockfile, err := CacheThunkPath(ctx, thunkPath)
	if err != nil {
		return nil, fmt.Errorf("cache %s: %w", thunkPath, err)
	}

	lockContent, err := os.ReadFile(cacheLockfile)
	if err != nil {
		return nil, fmt.Errorf("read memos: %w", err)
	}

	var content LockfileContent
	err = json.Unmarshal(lockContent, &content)
	if err != nil {
		return nil, fmt.Errorf("unmarshal memos: %w", err)
	}

	return ReadonlyMemos{content}, nil
}

type ReadonlyMemos struct {
	Content LockfileContent
}

var _ Memos = &ReadonlyMemos{}

func (file ReadonlyMemos) Store(thunk Thunk, binding Symbol, input Value, output Value) error {
	return nil
}

func (file ReadonlyMemos) Retrieve(thunk Thunk, binding Symbol, input Value) (Value, bool, error) {
	key, err := memoKey(thunk, binding)
	if err != nil {
		return nil, false, err
	}

	entries, found := file.Content.Data[key]
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

func (file ReadonlyMemos) Remove(thunk Thunk, binding Symbol, input Value) error {
	return nil
}

type WriteonlyMemos struct {
	Writer Memos
}

var _ Memos = &WriteonlyMemos{}

func (file WriteonlyMemos) Store(thunk Thunk, binding Symbol, input Value, output Value) error {
	return file.Writer.Store(thunk, binding, input, output)
}

func (file WriteonlyMemos) Retrieve(thunk Thunk, binding Symbol, input Value) (Value, bool, error) {
	return nil, false, nil
}

func (file WriteonlyMemos) Remove(thunk Thunk, binding Symbol, input Value) error {
	return file.Writer.Remove(thunk, binding, input)
}

func NewLockfileMemo(path string) *Lockfile {
	return &Lockfile{
		path: path,
		lock: flock.New(path),
	}
}

var _ Memos = &Lockfile{}

var globalLock = new(sync.RWMutex)

func (file *Lockfile) Store(thunk Thunk, binding Symbol, input Value, output Value) error {
	err := file.lock.Lock()
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}

	defer file.lock.Unlock()

	globalLock.Lock()
	defer globalLock.Unlock()

	content, err := file.load()
	if err != nil {
		return fmt.Errorf("load lock file: %w", err)
	}

	key, err := memoKey(thunk, binding)
	if err != nil {
		return err
	}

	entries, found := content.Data[key]
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

		sha, err := thunk.SHA256()
		if err != nil {
			return err
		}

		content.Thunks[sha] = thunk
	}

	content.Data[key] = entries

	return file.save(content)
}

func (file *Lockfile) Retrieve(thunk Thunk, binding Symbol, input Value) (Value, bool, error) {
	err := file.lock.RLock()
	if err != nil {
		return nil, false, fmt.Errorf("lock: %w", err)
	}

	defer file.lock.Unlock()

	globalLock.RLock()
	defer globalLock.RUnlock()

	content, err := file.load()
	if err != nil {
		return nil, false, fmt.Errorf("load lock file: %w", err)
	}

	key, err := memoKey(thunk, binding)
	if err != nil {
		return nil, false, err
	}

	entries, found := content.Data[key]
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

func (file *Lockfile) Remove(thunk Thunk, binding Symbol, input Value) error {
	err := file.lock.Lock()
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}

	defer file.lock.Unlock()

	globalLock.Lock()
	defer globalLock.Unlock()

	content, err := file.load()
	if err != nil {
		return fmt.Errorf("load lock file: %w", err)
	}

	key, err := memoKey(thunk, binding)
	if err != nil {
		return fmt.Errorf("memo key: %w", err)
	}

	entries, found := content.Data[key]
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
		delete(content.Data, key)
	} else {
		content.Data[key] = kept
	}

	return file.save(content)
}

func (file *Lockfile) load() (*LockfileContent, error) {
	payload, err := os.ReadFile(file.path)
	if err != nil {
		return nil, fmt.Errorf("read lock: %w", err)
	}

	content := LockfileContent{
		Data:   MemoData{},
		Thunks: map[string]Thunk{},
	}

	err = json.Unmarshal(payload, &content)
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

func (NoopMemos) Store(Thunk, Symbol, Value, Value) error {
	return nil
}

func (NoopMemos) Retrieve(Thunk, Symbol, Value) (Value, bool, error) {
	return nil, false, nil
}

func (NoopMemos) Remove(Thunk, Symbol, Value) error {
	return nil
}

func memoKey(thunk Thunk, binding Symbol) (string, error) {
	sha, err := thunk.SHA256()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", sha, binding), nil
}
