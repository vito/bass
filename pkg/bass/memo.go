package bass

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/gofrs/flock"
	"github.com/protocolbuffers/txtpbfmt/parser"
	"github.com/vito/bass/pkg/proto"
	"google.golang.org/protobuf/encoding/prototext"
	gproto "google.golang.org/protobuf/proto"
)

// Memos is where memoized calls are cached.
type Memos interface {
	Store(Thunk, Symbol, Value, Value) error
	Retrieve(Thunk, Symbol, Value) (Value, bool, error)
	Remove(Thunk, Symbol, Value) error
}

func init() {
	Ground.Set("recall-memo",
		Func("recall-memo", "[memos thunk binding input]", func(ctx context.Context, memos Readable, thunk Thunk, binding Symbol, input Value) (Value, error) {
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
		`See [memo] for the higher-level interface.`)

	Ground.Set("store-memo",
		Func("store-memo", "[memos thunk binding input result]", func(ctx context.Context, memos Readable, thunk Thunk, binding Symbol, input Value, res Value) (Value, error) {
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
		`See [memo] for the higher-level interface.`)
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

	content := &proto.Memosphere{}
	err = prototext.Unmarshal(lockContent, content)
	if err != nil {
		return nil, err
	}

	return ReadonlyMemos{content}, nil
}

type ReadonlyMemos struct {
	Content *proto.Memosphere
}

var _ Memos = &ReadonlyMemos{}

func (file ReadonlyMemos) Store(thunk Thunk, binding Symbol, input Value, output Value) error {
	return nil
}

func (file ReadonlyMemos) Retrieve(thunk Thunk, binding Symbol, input Value) (Value, bool, error) {
	return retrieveMemo(file.Content, thunk, binding, input)
}

func retrieveMemo(content *proto.Memosphere, thunk Thunk, binding Symbol, input Value) (Value, bool, error) {
	tp, err := thunk.Proto()
	if err != nil {
		return nil, false, err
	}

	im, err := MarshalProto(input)
	if err != nil {
		return nil, false, err
	}

	for _, memo := range content.Memos {
		if !gproto.Equal(memo.Module, tp) {
			continue
		}

		for _, call := range memo.Calls {
			if call.Binding != binding.String() {
				continue
			}

			for _, res := range call.Results {
				if !gproto.Equal(res.Input, im) {
					continue
				}

				val, err := FromProto(res.Output)
				if err != nil {
					return nil, false, err
				}

				return val, true, nil
			}
		}
	}

	return nil, false, nil
}

func (file ReadonlyMemos) Remove(thunk Thunk, binding Symbol, input Value) error {
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

	tp, err := thunk.Proto()
	if err != nil {
		return err
	}

	ip, err := MarshalProto(input)
	if err != nil {
		return err
	}

	op, err := MarshalProto(output)
	if err != nil {
		return err
	}

	var foundMod, foundCall, updated bool
	for _, memo := range content.Memos {
		if !gproto.Equal(memo.Module, tp) {
			continue
		}

		foundMod = true

		for _, call := range memo.Calls {
			if call.Binding != binding.String() {
				continue
			}

			foundCall = true

			for _, res := range call.Results {
				if !gproto.Equal(res.Input, ip) {
					continue
				}

				updated = true

				res.Output = op
			}

			if !updated {
				call.Results = append(call.Results, &proto.Memosphere_Result{
					Input:  ip,
					Output: op,
				})
			}
		}

		if !foundCall {
			memo.Calls = append(memo.Calls, &proto.Memosphere_Call{
				Binding: binding.String(),
				Results: []*proto.Memosphere_Result{
					{
						Input:  ip,
						Output: op,
					},
				},
			})
		}
	}

	if !foundMod {
		content.Memos = append(content.Memos, &proto.Memosphere_Memo{
			Module: tp,
			Calls: []*proto.Memosphere_Call{
				{
					Binding: binding.String(),
					Results: []*proto.Memosphere_Result{
						{
							Input:  ip,
							Output: op,
						},
					},
				},
			},
		})
	}

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

	return retrieveMemo(content, thunk, binding, input)
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

	tp, err := thunk.Proto()
	if err != nil {
		return err
	}

	im, err := MarshalProto(input)
	if err != nil {
		return err
	}

	keptMemos := make([]*proto.Memosphere_Memo, 0, len(content.Memos))
	for _, memo := range content.Memos {
		if !gproto.Equal(memo.Module, tp) {
			keptMemos = append(keptMemos, memo)
			continue
		}

		keptCalls := []*proto.Memosphere_Call{}
		for _, call := range memo.Calls {
			if call.Binding != binding.String() {
				keptCalls = append(keptCalls, call)
				continue
			}

			keptResults := []*proto.Memosphere_Result{}
			for _, res := range call.Results {
				if !gproto.Equal(res.Input, im) {
					keptResults = append(keptResults, res)
				}
			}

			if len(keptResults) > 0 {
				keptCalls = append(keptCalls, call)
			}

			call.Results = keptResults
		}

		memo.Calls = keptCalls

		if len(keptCalls) > 0 {
			keptMemos = append(keptMemos, memo)
		}
	}

	content.Memos = keptMemos

	return file.save(content)
}

func (file *Lockfile) load() (*proto.Memosphere, error) {
	payload, err := os.ReadFile(file.path)
	if err != nil {
		return nil, fmt.Errorf("read lock: %w", err)
	}

	content := &proto.Memosphere{}
	err = prototext.Unmarshal(payload, content)
	if err != nil {
		if errors.Is(err, gproto.Error) {
			return content, nil
		}

		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return content, nil
}

func (file *Lockfile) save(content *proto.Memosphere) error {
	payload, err := (prototext.MarshalOptions{Multiline: true}).Marshal(content)
	if err != nil {
		return err
	}

	fmted, err := parser.Format(payload)
	if err != nil {
		return err
	}

	return os.WriteFile(file.path, fmted, 0644)
}
