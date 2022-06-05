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
	"github.com/vito/bass/pkg/proto"
	"google.golang.org/protobuf/encoding/protojson"
	gproto "google.golang.org/protobuf/proto"
)

// Memos is where memoized calls are cached.
type Memos interface {
	Store(*proto.Thunk, Symbol, Value, Value) error
	Retrieve(*proto.Thunk, Symbol, Value) (Value, bool, error)
	Remove(*proto.Thunk, Symbol, Value) error
}

func init() {
	Ground.Set("recall-memo",
		Func("recall-memo", "[memos thunk binding input]", func(ctx context.Context, memos Readable, thunk Thunk, binding Symbol, input Value) (Value, error) {
			memo, err := OpenMemos(ctx, memos)
			if err != nil {
				return nil, fmt.Errorf("open memos at %s: %w", memos, err)
			}

			p, err := thunk.Proto()
			if err != nil {
				return nil, fmt.Errorf("proto: %w", err)
			}

			res, found, err := memo.Retrieve(p, binding, input)
			if err != nil {
				return nil, fmt.Errorf("retrieve memo %s:%s: %w", thunk.Repr(), binding, err)
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
		Func("store-memo", "[memos thunk binding input result]", func(ctx context.Context, memos Readable, thunk Thunk, binding Symbol, input Value, res Value) (Value, error) {
			memo, err := OpenMemos(ctx, memos)
			if err != nil {
				return nil, fmt.Errorf("open memos at %s: %w", memos, err)
			}

			p, err := thunk.Proto()
			if err != nil {
				return nil, fmt.Errorf("proto: %w", err)
			}

			err = memo.Store(p, binding, input, res)
			if err != nil {
				return nil, fmt.Errorf("store memo %s:%s: %w", thunk.Repr(), binding, err)
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

func OpenMemos(ctx context.Context, readable Readable) (Memos, error) {
	cacheLockfile, err := readable.CachePath(ctx, CacheHome)
	if err != nil {
		return nil, fmt.Errorf("cache %s: %w", readable, err)
	}

	var hostPath HostPath
	if err := readable.Decode(&hostPath); err == nil {
		return NewLockfileMemo(cacheLockfile), nil
	}

	lockContent, err := os.ReadFile(cacheLockfile)
	if err != nil {
		return nil, fmt.Errorf("read memos: %w", err)
	}

	var content proto.Memosphere
	err = protojson.Unmarshal(lockContent, &content)
	if err != nil {
		return nil, fmt.Errorf("unmarshal memos: %w", err)
	}

	return ReadonlyMemos{&content}, nil
}

type ReadonlyMemos struct {
	Content *proto.Memosphere
}

var _ Memos = &ReadonlyMemos{}

func (file ReadonlyMemos) Store(thunk *proto.Thunk, binding Symbol, input Value, output Value) error {
	return nil
}

func (file ReadonlyMemos) Retrieve(thunk *proto.Thunk, binding Symbol, input Value) (Value, bool, error) {
	key, err := memoKey(thunk, binding)
	if err != nil {
		return nil, false, err
	}

	entries, found := file.Content.Data[key]
	if !found {
		return nil, false, nil
	}

	im, err := MarshalProto(input)
	if err != nil {
		return nil, false, err
	}

	for _, e := range entries.GetMemories() {
		if gproto.Equal(e.Input, im) {
			return nil, true, nil // TODO
		}
	}

	return nil, false, nil
}

func (file ReadonlyMemos) Remove(thunk *proto.Thunk, binding Symbol, input Value) error {
	return nil
}

func NewLockfileMemo(path string) *Lockfile {
	return &Lockfile{
		path: path,
		lock: flock.New(path),
	}
}

var _ Memos = &Lockfile{}

var globalLock = new(sync.RWMutex)

func (file *Lockfile) Store(thunk *proto.Thunk, binding Symbol, input Value, output Value) error {
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

	var entries []*proto.Memory
	memories, found := content.Data[key]
	if found {
		entries = memories.Memories
	}

	ip, err := MarshalProto(input)
	if err != nil {
		return err
	}

	op, err := MarshalProto(output)
	if err != nil {
		return err
	}

	var updated bool
	for i, e := range entries {
		if gproto.Equal(e.Input, ip) {
			entries[i].Output = op
			updated = true
		}
	}

	if !updated {
		entries = append(entries, &proto.Memory{
			Input:  ip,
			Output: op,
		})

		sha, err := thunk.SHA256()
		if err != nil {
			return err
		}

		content.Modules[sha] = thunk
	}

	content.Data[key].Memories = entries

	return file.save(content)
}

func (file *Lockfile) Retrieve(thunk *proto.Thunk, binding Symbol, input Value) (Value, bool, error) {
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

	ip, err := MarshalProto(input)
	if err != nil {
		return nil, false, err
	}

	for _, e := range entries.GetMemories() {
		if gproto.Equal(e.Input, ip) {
			return nil, true, nil // TODO
		}
	}

	return nil, false, nil
}

func (file *Lockfile) Remove(thunk *proto.Thunk, binding Symbol, input Value) error {
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

	ip, err := MarshalProto(input)
	if err != nil {
		return err
	}

	kept := []*proto.Memory{}
	for _, e := range entries.GetMemories() {
		// TODO: would be nice to support IsSubsetOf semantics
		if !gproto.Equal(ip, e.Input) {
			kept = append(kept, e)
		}
	}

	if len(kept) == 0 {
		delete(content.Data, key)
	} else {
		content.Data[key].Memories = kept
	}

	return file.save(content)
}

func (file *Lockfile) load() (*proto.Memosphere, error) {
	payload, err := os.ReadFile(file.path)
	if err != nil {
		return nil, fmt.Errorf("read lock: %w", err)
	}

	var content proto.Memosphere
	err = protojson.Unmarshal(payload, &content)
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
		filtered := []*proto.Memory{}
		for _, e := range es.GetMemories() {
			if e.Input.Value == nil || e.Output.Value == nil {
				// filter any corrupt entries
				continue
			}

			filtered = append(filtered, e)
		}

		if len(filtered) == 0 {
			delete(content.Data, c)
		} else {
			content.Data[c].Memories = filtered
		}
	}

	return &content, nil
}

func (file *Lockfile) save(content *proto.Memosphere) error {
	buf := new(bytes.Buffer)
	enc := NewEncoder(buf)
	enc.SetIndent("", "  ")

	err := enc.Encode(content)
	if err != nil {
		return err
	}

	return os.WriteFile(file.path, buf.Bytes(), 0644)
}

func memoKey(thunk *proto.Thunk, binding Symbol) (string, error) {
	sha, err := thunk.SHA256()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", sha, binding), nil
}
