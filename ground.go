package bass

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/std"
	"github.com/vito/bass/zapctx"
)

// Ground is the scope providing the standard library.
var Ground = NewEmptyScope()

// Clock is used to determine the current time.
var Clock = clockwork.NewRealClock()

// NewStandardScope returns a new empty scope with Ground as its sole parent.
func NewStandardScope() *Scope {
	return NewEmptyScope(Ground)
}

func init() {
	Ground.Name = "ground"

	Ground.Comment(Ignore{},
		"This module bootstraps the ground scope with basic language facilities.")

	Ground.Set("def",
		Op("def", "[binding value]", func(ctx context.Context, cont Cont, scope *Scope, formals Bindable, val Value) ReadyCont {
			return val.Eval(ctx, scope, Continue(func(res Value) Value {
				err := formals.Bind(scope, res)
				if err != nil {
					return cont.Call(nil, err)
				}

				return cont.Call(formals, nil)
			}))
		}),
		`bind symbols to values in the current scope`)

	for _, pred := range primPreds {
		Ground.Set(pred.name, Func(string(pred.name), "[val]", pred.check), pred.docs...)
	}

	Ground.Set("ground", Ground, `ground scope please ignore`,
		`This value is only here to aid in developing prior to first release.`,
		`Fetching this binding voids your warranty.`)

	Ground.Set("dump",
		Func("dump", "[val]", func(ctx context.Context, val Value) Value {
			Dump(ioctx.StderrFromContext(ctx), val)
			return val
		}),
		`writes a value as JSON to stderr`,
		`Returns the given value.`)

	Ground.Set("log",
		Func("log", "[val]", func(ctx context.Context, v Value) Value {
			var msg string
			if err := v.Decode(&msg); err == nil {
				zapctx.FromContext(ctx).Info(msg)
			} else {
				zapctx.FromContext(ctx).Info(v.String())
			}

			return v
		}),
		`writes a string message or other arbitrary value to stderr`,
		`Returns the given value.`)

	Ground.Set("logf",
		Func("logf", "[fmt & args]", func(ctx context.Context, msg string, args ...Value) {
			zapctx.FromContext(ctx).Sugar().Infof(msg, fmtArgs(args...)...)
		}),
		`writes a message formatted with the given values`)

	Ground.Set("time",
		Op("time", "[form]", func(ctx context.Context, cont Cont, scope *Scope, form Value) ReadyCont {
			before := Clock.Now()
			return form.Eval(ctx, scope, Continue(func(res Value) Value {
				took := time.Since(before)
				zapctx.FromContext(ctx).Sugar().Debugf("(time %s) => %s took %s", form, res, took)
				return cont.Call(res, nil)
			}))
		}),
		`evaluates the form and prints the time it took`,
		`Returns the value returned by the form.`)

	Ground.Set("now",
		Func("now", "[duration]", func(duration int) string {
			return Clock.Now().Truncate(time.Duration(duration) * time.Second).UTC().Format(time.RFC3339)
		}),
		`returns the current UTC time truncated to the given duration (in seconds)`,
		`Typically used to annotate workloads whose result may change over time.`,
		`By specifying a duration, these workloads can still be cached to a configurable level of granularity.`,
		`=> (now 60)`)

	Ground.Set("error",
		Func("error", "[msg]", errors.New),
		`errors with the given message`)

	Ground.Set("errorf",
		Func("errorf", "[fmt & args]", func(msg string, args ...Value) error {
			return fmt.Errorf(msg, fmtArgs(args...)...)
		}),
		`errors with a message formatted with the given values`)

	Ground.Set("do",
		Op("do", "body", func(ctx context.Context, cont Cont, scope *Scope, body ...Value) ReadyCont {
			return do(ctx, cont, scope, body)
		}),
		`evaluate a sequence, returning the last value`)

	Ground.Set("cons",
		Func("cons", "[a d]", func(a, d Value) Value {
			return Pair{a, d}
		}),
		`construct a pair from the given values`)

	Ground.Set("wrap",
		Func("wrap", "[comb]", Wrap),
		`construct an applicative from a combiner (typically an operative)`)

	Ground.Set("unwrap",
		Func("unwrap", "[app]", func(a Applicative) Combiner {
			return a.Unwrap()
		}),
		`access an applicative's underlying combiner`)

	Ground.Set("op",
		Op("op", "[formals eformal body]", func(scope *Scope, formals, eformal Bindable, body Value) *Operative {
			return &Operative{
				Scope:       scope,
				Formals:     formals,
				ScopeFormal: eformal,
				Body:        body,
			}
		}),
		// no commentary; it's redefined later
	)

	Ground.Set("eval",
		Func("eval", "[form scope]", func(ctx context.Context, cont Cont, val Value, scope *Scope) ReadyCont {
			return val.Eval(ctx, scope, cont)
		}),
		`evaluate a value in a scope`)

	Ground.Set("make-scope",
		Func("make-scope", "parents", NewEmptyScope),
		`construct a scope with the given parents`)

	Ground.Set("bind",
		Func("bind", "[scope formals val]", func(scope *Scope, formals Bindable, val Value) bool {
			err := formals.Bind(scope, val)
			return err == nil
		}),
		`attempts to bind values in the scope`,
		`Returns true if the binding succeeded, otherwise false.`)

	Ground.Set("doc",
		Op("doc", "symbols", PrintDocs),
		`print docs for symbols`,
		`Prints the documentation for the given symbols resolved from the current scope.`,
		`With no arguments, prints the commentary for the current scope.`)

	Ground.Set("comment",
		Op("comment", "[form annotated]", func(ctx context.Context, cont Cont, scope *Scope, form Value, doc Annotated) ReadyCont {
			annotated, ok := form.(Annotated)
			if ok {
				annotated.Comment = doc.Comment
				annotated.Range = doc.Range
			} else {
				doc.Value = form

				annotated = doc
			}

			return annotated.Eval(ctx, scope, cont)
		}),
		`record a comment`,
		`Splices one annotated value with another, recording commentary in the current scope.`,
		`Typically used by operatives to preserve commentary between scopes.`)

	Ground.Set("commentary",
		Op("commentary", "[sym]", func(scope *Scope, sym Symbol) Annotated {
			annotated, found := scope.Docs[sym]
			if !found {
				return Annotated{
					Value: sym,
				}
			}

			return annotated
		}),
		`return the comment string associated to the symbol`,
		`Typically used by operatives to preserve commentary between scopes.`,
		`Use (doc) instead for prettier output.`)

	Ground.Set("if",
		Op("if", "[cond yes no]", func(ctx context.Context, cont Cont, scope *Scope, cond, yes, no Value) ReadyCont {
			return cond.Eval(ctx, scope, Continue(func(res Value) Value {
				var b bool
				err := res.Decode(&b)
				if err != nil {
					return yes.Eval(ctx, scope, cont)
				}

				if b {
					return yes.Eval(ctx, scope, cont)
				} else {
					return no.Eval(ctx, scope, cont)
				}
			}))
		}),
		`if then else (branching logic)`,
		`Evaluates a condition. If nil or false, evaluates the third operand. Otherwise, evaluates the second operand.`)

	Ground.Set("+",
		Func("+", "nums", func(nums ...int) int {
			sum := 0
			for _, num := range nums {
				sum += num
			}

			return sum
		}),
		`sum the given numbers`)

	Ground.Set("*",
		Func("*", "nums", func(nums ...int) int {
			mul := 1
			for _, num := range nums {
				mul *= num
			}

			return mul
		}),
		`multiply the given numbers`)

	Ground.Set("-",
		Func("-", "[num & nums]", func(num int, nums ...int) int {
			if len(nums) == 0 {
				return -num
			}

			sub := num
			for _, num := range nums {
				sub -= num
			}

			return sub
		}),
		`subtract ys from x`,
		`If only x is given, returns the negation of x.`)

	Ground.Set("max",
		Func("max", "[num & nums]", func(num int, nums ...int) int {
			max := num
			for _, num := range nums {
				if num > max {
					max = num
				}
			}

			return max
		}),
		`largest number given`)

	Ground.Set("min",
		Func("min", "[num & nums]", func(num int, nums ...int) int {
			min := num
			for _, num := range nums {
				if num < min {
					min = num
				}
			}

			return min
		}),
		`smallest number given`)

	Ground.Set("=",
		Func("=", "[val & vals]", func(val Value, others ...Value) bool {
			for _, other := range others {
				if !other.Equal(val) {
					return false
				}
			}

			return true
		}),
		`numeric equality`,
	)

	Ground.Set(">",
		Func(">", "[num & nums]", func(num int, nums ...int) bool {
			min := num
			for _, num := range nums {
				if num >= min {
					return false
				}

				min = num
			}

			return true
		}),
		`descending order`)

	Ground.Set(">=",
		Func(">=", "[num & nums]", func(num int, nums ...int) bool {
			max := num
			for _, num := range nums {
				if num > max {
					return false
				}

				max = num
			}

			return true
		}),
		`descending or equal order`)

	Ground.Set("<",
		Func("<", "[num & nums]", func(num int, nums ...int) bool {
			max := num
			for _, num := range nums {
				if num <= max {
					return false
				}

				max = num
			}

			return true
		}),
		`increasing order`)

	Ground.Set("<=",
		Func("<=", "[num & nums]", func(num int, nums ...int) bool {
			max := num
			for _, num := range nums {
				if num < max {
					return false
				}

				max = num
			}

			return true
		}),
		`increasing or equal order`)

	Ground.Set("*stdin*", Stdin, "A source? of values read from stdin.")
	Ground.Set("*stdout*", Stdout, "A sink? for writing values to stdout.")

	Ground.Set("stream",
		Func("stream", "vals", func(vals ...Value) Value {
			return &Source{NewInMemorySource(vals...)}
		}),
		"construct a stream source for a sequence of values")

	Ground.Set("emit",
		Func("emit", "[val sink]", func(val Value, sink PipeSink) error {
			return sink.Emit(val)
		}),
		`send a value to a sink`,
	)

	Ground.Set("next",
		Func("next", "[src & default]", func(ctx context.Context, source PipeSource, def ...Value) (Value, error) {
			val, err := source.Next(ctx)
			if err != nil {
				if errors.Is(err, ErrEndOfSource) && len(def) > 0 {
					return def[0], nil
				}

				return nil, err
			}

			return val, nil
		}),
		`receive the next value from a source`,
		`If the stream has ended, no value will be available. A default value may be provided, otherwise an error is raised.`,
	)

	Ground.Set("reduce-kv",
		Wrapped{Op("reduce-kv", "[f init kv]", func(ctx context.Context, scope *Scope, fn Applicative, init Value, kv *Scope) (Value, error) {
			op := fn.Unwrap()

			res := init
			err := kv.Each(func(k Symbol, v Value) error {
				// XXX: this drops trace info, i think; refactor into CPS

				var err error
				res, err = Trampoline(ctx, op.Call(ctx, NewConsList(res, k, v), scope, Identity))
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				return nil, err
			}

			return res, nil
		})},
		`reduces a scope`,
		`Takes a 3-arity function, an initial value, and a scope. If the scope is empty, the initial value is returned. Otherwise, calls the function for each key-value pair, with the current value as the first argument.`,
		`=> (reduce-kv assoc {:d 4} {:a 1 :b 2 :c 3})`,
	)

	Ground.Set("assoc",
		Func("assoc", "[obj & kvs]", func(obj *Scope, kv ...Value) (*Scope, error) {
			clone := obj.Clone()

			var k Symbol
			var v Value
			for i := 0; i < len(kv); i++ {
				if i%2 == 0 {
					err := kv[i].Decode(&k)
					if err != nil {
						return nil, err
					}
				} else {
					err := kv[i].Decode(&v)
					if err != nil {
						return nil, err
					}

					clone.Set(k, v)

					k = ""
					v = nil
				}
			}

			return clone, nil
		}),
		`assoc[iate] keys with values in a clone of a scope`,
		`Takes a scope and a flat pair sequence alternating symbols and values.`,
		`Returns a clone of the scope with the symbols fields set to their associated value.`,
	)

	Ground.Set("symbol->string",
		Func("symbol->string", "[sym]", func(sym Symbol) String {
			return String(sym)
		}),
		`convert a symbol to a string`)

	Ground.Set("string->symbol",
		Func("string->symbol", "[str]", func(str String) Symbol {
			return Symbol(str)
		}),
		`convert a string to a symbol`)

	Ground.Set("str",
		Func("str", "vals", func(vals ...Value) String {
			var str string = ""

			for _, v := range vals {
				var s string
				if err := v.Decode(&s); err == nil {
					str += s
				} else {
					str += v.String()
				}
			}

			return String(str)
		}),
		`returns the concatenation of all given strings or values`)

	Ground.Set("substring",
		Func("substring", "[str start & end]", func(str String, start Int, endOptional ...Int) (String, error) {
			switch len(endOptional) {
			case 0:
				return str[start:], nil
			case 1:
				return str[start:endOptional[0]], nil
			default:
				// TODO: test
				return "", ArityError{
					Name: "substring",
					Need: 3,
					Have: 4,
				}
			}
		}),
		`returns a portion of a string`,
		`With one number supplied, returns the portion from the offset to the end.`,
		`With two numbers supplied, returns the portion between the first offset and the last offset, exclusive.`)

	Ground.Set("scope->list",
		Func("scope->list", "[obj]", func(obj *Scope) List {
			var vals []Value
			_ = obj.Each(func(k Symbol, v Value) error {
				vals = append(vals, k, v)
				return nil
			})

			return NewList(vals...)
		}),
		`returns a flat list alternating a scope's keys and values`,
		`The returned list is the same form accepted by (map-pairs).`)

	Ground.Set("string->path",
		Func("string->path", "[str]", ParseFilesystemPath))

	Ground.Set("string->run-path",
		Func("string->run-path", "[str]", func(s string) (Path, error) {
			if !strings.Contains(s, "/") {
				return CommandPath{s}, nil
			}

			return ParseFilesystemPath(s)
		}))

	Ground.Set("string->dir",
		Func("string->dir", "[str]", func(s string) (DirPath, error) {
			fspath, err := ParseFilesystemPath(s)
			if err != nil {
				return DirPath{}, err
			}

			if fspath.IsDir() {
				return fspath.(DirPath), nil
			} else {
				return DirPath{
					Path: fspath.(FilePath).Path,
				}, nil
			}
		}))

	Ground.Set("merge",
		Func("merge", "[obj & objs]", func(obj *Scope, objs ...*Scope) *Scope {
			merged := obj.Clone()
			for _, o := range objs {
				_ = o.Each(func(k Symbol, v Value) error {
					merged.Set(k, v)
					return nil
				})
			}
			return merged
		}))

	Ground.Set("load",
		Func("load", "[workload]", func(ctx context.Context, workload Workload) (*Scope, error) {
			runtime, err := RuntimeFromContext(ctx)
			if err != nil {
				return nil, err
			}

			return runtime.Load(ctx, workload)
		}))

	Ground.Set("run",
		Func("run", "[workload]", func(ctx context.Context, workload Workload) (*Source, error) {
			runtime, err := RuntimeFromContext(ctx)
			if err != nil {
				return nil, err
			}

			buf := new(bytes.Buffer)
			err = runtime.Run(ctx, buf, workload)
			if err != nil {
				return nil, err
			}

			return NewSource(NewJSONSource(workload.String(), buf)), nil
		}),
		`run a workload`)

	Ground.Set("path",
		Func("path", "[workload path]", func(ctx context.Context, workload Workload, path FileOrDirPath) WorkloadPath {
			return WorkloadPath{
				Workload: workload,
				Path:     path,
			}
		}),
		`returns a path within a workload`)

	for _, lib := range []string{
		"root.bass",
		"lists.bass",
		"streams.bass",
		"run.bass",
		"bool.bass",
	} {
		file, err := std.FS.Open(lib)
		if err != nil {
			panic(err)
		}

		_, err = EvalReader(context.Background(), Ground, file, lib)
		if err != nil {
			fmt.Fprintf(os.Stderr, "eval ground %s: %s\n", lib, err)
		}

		_ = file.Close()
	}
}

type primPred struct {
	name  Symbol
	check func(Value) bool
	docs  []string
}

// basic predicates built in to the language.
//
// these are also used in (doc) to show which predicates a value satisfies.
var primPreds = []primPred{
	// primitive type checks
	{"null?", func(val Value) bool {
		var x Null
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is null`}},

	{"ignore?", func(val Value) bool {
		var x Ignore
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is _ ("ignore")`,
		`_ is a special value used to ignore a value when binding symbols.`,
		`=> (let [(fst & _) [1 2]] fst)`,
	}},

	{"boolean?", func(val Value) bool {
		var x Bool
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is true or false`}},

	{"number?", func(val Value) bool {
		var x Int
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a number`}},

	{"string?", func(val Value) bool {
		var x String
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a string`}},

	{"symbol?", func(val Value) bool {
		var x Symbol
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a symbol`}},

	{"scope?", func(val Value) bool {
		var x *Scope
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a scope`}},

	{"sink?", func(val Value) bool {
		var x *Sink
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a sink`,
		`A sink is a type that you can send values to using (emit).`,
	}},

	{"source?", func(val Value) bool {
		var x *Source
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a source`,
		`A source is a type that you can read values from using (next).`,
	}},

	{"list?", IsList, []string{
		`returns true if the value is a linked list`,
		`A linked list is a pair whose second value is another list or empty.`,
	}},

	{"pair?", func(val Value) bool {
		var x Pair
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a pair`}},

	{"scope?", func(val Value) bool {
		var x *Scope
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a scope`,
		`A scope is a mapping from symbols to values.`,
	}},

	{"applicative?", func(val Value) bool {
		var x Applicative
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is an applicative`,
		`An applicative is a combiner that wraps another combiner.`,
		`When an applicative is called, it evaluates its operands in the caller's evironment and passes them to the underlying combiner.`,
	}},

	{"operative?", func(val Value) bool {
		var b *Builtin
		if val.Decode(&b) == nil {
			return b.Operative
		}

		var o *Operative
		return val.Decode(&o) == nil
	}, []string{`returns true if the value is an operative`,
		`An operative is a combiner that is given the caller's scope.`,
		`An operative may decide whether and how to evaluate its arguments. They are typically used to define new syntactic constructs.`,
	}},

	{"combiner?", func(val Value) bool {
		var x Combiner
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a combiner`,
		`A combiner takes sequence of values as arguments and returns another value.`,
	}},

	{"path?", func(val Value) bool {
		var x Path
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a path`,
		`A path is a reference to a file, directory, or command.`,
	}},

	{"empty?", func(val Value) bool {
		var bind Bind
		if err := val.Decode(&bind); err == nil {
			return len(bind) == 0
		}

		var empty Empty
		if err := val.Decode(&empty); err == nil {
			return true
		}

		var str string
		if err := val.Decode(&str); err == nil {
			return str == ""
		}

		var scope *Scope
		if err := val.Decode(&scope); err == nil {
			return scope.IsEmpty()
		}

		var nul Null
		if err := val.Decode(&nul); err == nil {
			return true
		}

		return false
	}, []string{
		`returns true if the value is an empty list, a zero-length string, an empty scope, or null`,
	}},
}

func fmtArgs(args ...Value) []interface{} {
	is := make([]interface{}, len(args))
	for i := range args {
		var s string
		if err := args[i].Decode(&s); err == nil {
			is[i] = s
		} else {
			is[i] = args[i]
		}
	}

	return is
}

func do(ctx context.Context, cont Cont, scope *Scope, body []Value) ReadyCont {
	if len(body) == 0 {
		return cont.Call(Null{}, nil)
	}

	next := cont
	if len(body) > 1 {
		next = Continue(func(res Value) Value {
			return do(ctx, cont, scope, body[1:])
		})
	}

	return body[0].Eval(ctx, scope, next)
}
