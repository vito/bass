package bass

import (
	"archive/tar"
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
		Func("memo", "[f memos category]", func(f Combiner, memos Path, category Symbol) Combiner {
			return Wrap(Op("memo", "[selector]", func(ctx context.Context, cont Cont, scope *Scope, args ...Value) ReadyCont {
				memo, err := OpenMemos(ctx, memos)
				if err != nil {
					return cont.Call(nil, fmt.Errorf("open memos at %s: %w", memos, err))
				}

				input := NewList(args...)

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
		}),
		`memo[ize]s a function`,
		`This is a utility for caching dependency version resolution, such as image tags and git refs. It is technically the only way to perform writes against the host filesystem.`,
		`Returns a function which will cache its results in memos under the given category.`,
		`If memos is a dir, searches in the directory traversing upwards until a bass.lock file is found.`,
		`If memos is a file, no searching is performed. If memos is a host path, the file will be created if it does not exist.`,
		`The intended practice is to commit the bass.lock file into source control to facilitate reproducible builds.`)
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
	if hostPath.Path.FilesystemPath().IsDir() {
		if lf, ok := searchLockfile(hostPath.FromSlash()); ok {
			return lf
		} else {
			return NoopMemos{}
		}
	} else {
		return NewLockfileMemo(hostPath.FromSlash())
	}
}

func OpenFSPathMemos(fsPath FSPath) (Memos, error) {
	if fsPath.Path.FilesystemPath().IsDir() {
		searchPath := fsPath

		lf := FilePath{LockfileName}

		for {
			lfPath, err := searchPath.Path.FilesystemPath().Extend(lf)
			if err != nil {
				// should be impossible given that it's IsDir
				return nil, err
			}

			fsp := lfPath.(FilesystemPath)

			searchPath.Path = NewFileOrDirPath(fsp)
			memos, err := OpenFSPathMemos(searchPath)
			if err != nil {
				parent := fsp.Dir().Dir()
				if parent.Equal(fsp.Dir()) {
					return NoopMemos{}, nil
				}

				searchPath.Path = NewFileOrDirPath(parent)
				continue
			}

			return memos, nil
		}
	} else {
		file, err := fsPath.FS.Open(fsPath.Path.File.Path)
		if err != nil {
			return nil, err
		}

		defer file.Close()

		dec := NewDecoder(file)

		var content LockfileContent
		err = dec.Decode(&content)
		if err != nil {
			return nil, err
		}

		return ReadonlyMemos{content}, nil
	}
}

func OpenThunkPathMemos(ctx context.Context, thunkPath ThunkPath) (Memos, error) {
	pool, err := RuntimePoolFromContext(ctx)
	if err != nil {
		return nil, err
	}

	runtime, err := pool.Select(thunkPath.Thunk.Platform())
	if err != nil {
		return nil, err
	}

	if thunkPath.Path.FilesystemPath().IsDir() {
		searchPath := thunkPath

		// HACK: want to use a wildcard just so we can allow an empty result, but still
		// want an exact match; this should do the trick
		lf := FilePath{"bass.l[o]ck"}

		for {
			lfPath, err := searchPath.Path.FilesystemPath().Extend(lf)
			if err != nil {
				// should be impossible given that it's IsDir
				return nil, err
			}

			fsp := lfPath.(FilesystemPath)

			searchPath.Path = NewFileOrDirPath(fsp)

			buf := new(bytes.Buffer)
			err = runtime.ExportPath(ctx, buf, searchPath)
			if err != nil {
				return nil, err
			}

			tr := tar.NewReader(buf)

			_, err = tr.Next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					parent := fsp.Dir().Dir()
					if parent.Equal(fsp.Dir()) {
						return NoopMemos{}, nil
					}

					searchPath.Path = NewFileOrDirPath(parent)
					continue
				}

				return nil, fmt.Errorf("tar next: %w", err)
			}

			var content LockfileContent
			dec := NewDecoder(tr)
			err = dec.Decode(&content)
			if err != nil {
				return nil, fmt.Errorf("unmarshal memos: %w", err)
			}

			return ReadonlyMemos{content}, nil
		}
	} else {
		buf := new(bytes.Buffer)
		err := runtime.ExportPath(ctx, buf, thunkPath)
		if err != nil {
			return nil, err
		}
		tr := tar.NewReader(buf)

		_, err = tr.Next()
		if err != nil {
			return nil, fmt.Errorf("tar next: %w", err)
		}

		var content LockfileContent
		dec := NewDecoder(tr)
		err = dec.Decode(&content)
		if err != nil {
			return nil, fmt.Errorf("unmarshal memos: %w", err)
		}

		return ReadonlyMemos{content}, nil
	}
}

type ReadonlyMemos struct {
	Content LockfileContent
}

var _ Memos = &Lockfile{}

func (file ReadonlyMemos) Store(category Symbol, input Value, output Value) error {
	return nil
}

func (file ReadonlyMemos) Retrieve(category Symbol, input Value) (Value, bool, error) {
	entries, found := file.Content.Data[category]
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

func (file ReadonlyMemos) Remove(category Symbol, input Value) error {
	return nil
}

type WriteonlyMemos struct {
	Writer Memos
}

var _ Memos = &Lockfile{}

func (file WriteonlyMemos) Store(category Symbol, input Value, output Value) error {
	return file.Writer.Store(category, input, output)
}

func (file WriteonlyMemos) Retrieve(category Symbol, input Value) (Value, bool, error) {
	return nil, false, nil
}

func (file WriteonlyMemos) Remove(category Symbol, input Value) error {
	return file.Writer.Remove(category, input)
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
