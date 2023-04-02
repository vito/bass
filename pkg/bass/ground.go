package bass

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/jonboulle/clockwork"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/zapctx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Ground is the scope providing the standard library.
var Ground = NewEmptyScope()

// Clock is used to determine the current time.
var Clock = clockwork.NewRealClock()

// Bass is the canonical global session for running Bass scripts.
var Bass = NewBass()

// NewStandardScope returns a new empty scope with Ground as its sole parent.
func NewStandardScope() *Scope {
	return NewEmptyScope(Ground)
}

func init() {
	Ground.Name = "ground"

	Ground.Set("def",
		Op("def", "[binding value]", func(ctx context.Context, cont Cont, scope *Scope, formals Bindable, val Value) ReadyCont {
			return val.Eval(ctx, scope, Continue(func(res Value) Value {
				return formals.Bind(ctx, scope, cont, res)
			}))
		}),
		`bind symbols to values in the current scope`,
		`Supports destructuring assignment.`,
		`=> (def abc "it's easy as")`,
		`=> (def [a b c] [1 2 3])`,
		`=> [abc a b c]`)

	Ground.Set("if",
		Annotated{
			Value: Op("if", "[cond yes no]", func(ctx context.Context, cont Cont, scope *Scope, cond, yes, no Value) ReadyCont {
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
			Meta: Bindings{"indent": Bool(true)}.Scope(),
		},
		`if then else (branching logic)`,
		`Evaluates the cond form. If the result is truthy (not false or null), evaluates the yes form. Otherwise, evaluates the no form.`,
		`=> (if false (error "bam") :phew)`)

	Ground.Set("dump",
		Func("dump", "[val]", func(ctx context.Context, val Value) Value {
			Dump(ioctx.StderrFromContext(ctx), val)
			return val
		}),
		`encodes a value as JSON to stderr`,
		`Returns the given value.`,
		`=> (dump {:foo-bar "baz"})`)

	Ground.Set("mkfs",
		Func("mkfs", "file-content-kv", NewInMemoryFSDir),
		`returns a dir path backed by an in-memory filesystem`,
		`Takes alternating file paths and their content, which must be a text string, and returns the root directory of an in-memory filesystem containing the specified files.`,
		`All embedded files have 0644 Unix file permissions and a zero (Unix epoch) mtime.`,
		`=> (def fs (mkfs ./file "hey" ./sub/file "im in a subdir"))`,
		`=> (next (read (from (linux/alpine) ($ cat fs/file)) :raw))`,
	)

	Ground.Set("json",
		Func("json", "[val]", func(ctx context.Context, val Value) (string, error) {
			payload, err := MarshalJSON(val)
			if err != nil {
				return "", err
			}

			return string(payload), nil
		}),
		`returns a string containing val encoded as JSON`,
		`=> (json {:foo-bar "baz"})`)

	Ground.Set("log",
		Func("log", "[val & fields]", func(ctx context.Context, v Value, kv ...Value) (Value, error) {
			logger := zapctx.FromContext(ctx)

			if len(kv) > 0 {
				fields, err := Assoc(NewEmptyScope(), kv...)
				if err != nil {
					return nil, err
				}

				err = fields.Each(func(k Symbol, v Value) error {
					f, err := zapField(k, v)
					if err != nil {
						return err
					}
					logger = logger.With(f)
					return nil
				})
				if err != nil {
					return nil, err
				}
			}

			var msg string
			if err := v.Decode(&msg); err == nil {
				logger.Info(msg)
			} else {
				logger.Info(v.String())
			}

			return v, nil
		}),
		`logs a string message or arbitrary value to stderr`,
		`Returns the given value.`,
		`Accepts key-value fields for structured logging data.`,
		`=> (log "hello, world!")`,
		`=> (log "doing something" :a 1 :since {:day 1})`)

	Ground.Set("error",
		Func("error", "[msg & fields]", NewError),
		`errors with the given message`,
		`Accepts key-value fields for structured error data.`,
		`=> (error "oh no!")`,
		`=> (error "oh no!" :exit-code 2)`)

	Ground.Set("now",
		Func("now", "[seconds]", func(duration int) string {
			return Clock.Now().Truncate(time.Duration(duration) * time.Second).UTC().Format(time.RFC3339)
		}),
		`returns the current UTC time truncated to the given seconds`,
		`Typically used to influence caching for thunks whose result may change over time.`,
		`=> (now 60)`)

	Ground.Set("do",
		Op("do", "body", func(ctx context.Context, cont Cont, scope *Scope, body ...Value) ReadyCont {
			return do(ctx, cont, scope, body)
		}),
		`evaluate a sequence, returning the last value`,
		`=> (do (def abc 123) (+ abc 1))`,
		`=> abc`)

	Ground.Set("cons",
		Func("cons", "[a d]", func(a, d Value) Value {
			return Pair{a, d}
		}),
		`construct a pair from the given values`,
		`=> (cons 1 [2 3])`,
		`=> (cons 1 2)`)

	Ground.Set("wrap",
		Func("wrap", "[comb]", Wrap),
		`construct an applicative from a combiner (typically an operative)`,
		`When called, an applicative evaluates its arguments before passing them along to the underlying combiner.`,
		`=> (defop log-quote [x] _ (log x) x)`,
		`=> (log-quote (* 6 7))`,
		`=> ((wrap log-quote) (* 6 7))`)

	Ground.Set("unwrap",
		Func("unwrap", "[app]", (Applicative).Unwrap),
		`returns an applicative's underlying combiner`,
		`You probably won't use this a lot. It's used to implement higher level abstractions like [apply].`)

	Ground.Set("op",
		Op("op", "[formals eformal body]", func(scope *Scope, formals, eformal Bindable, body Value) *Operative {
			return &Operative{
				StaticScope:  scope,
				Bindings:     formals,
				ScopeBinding: eformal,
				Body:         body,
			}
		}),
		// no commentary; it's redefined later
	)

	Ground.Set("eval",
		Func("eval", "[form scope]", func(ctx context.Context, cont Cont, val Value, scope *Scope) ReadyCont {
			return val.Eval(ctx, scope, cont)
		}),
		`evaluate a value in a scope`,
		`=> (eval :abc {:abc 123})`,
		`=> (eval [* :x :y] {:x 6 :y 7})`)

	Ground.Set("make-scope",
		Func("make-scope", "parents", NewEmptyScope),
		`construct a scope with the given parents`,
		`=> (make-scope {:a 1} {:b 2})`,
		`=> (eval [+ :a :b] (make-scope {:a 1} {:b 2}))`)

	Ground.Set("bind",
		Func("bind", "[scope formals val]", func(ctx context.Context, cont Cont, scope *Scope, formals Bindable, val Value) ReadyCont {
			// TODO: using a Trampoline here is a bit of a smell
			_, err := Trampoline(ctx, formals.Bind(ctx, scope, Identity, val))
			return cont.Call(Bool(err == nil), nil)
		}),
		`attempts to bind values in the scope`,
		`Returns true if the binding succeeded, otherwise false.`,
		`=> (if (bind (current-scope) :abc 123) abc :mismatch)`,
		`=> (if (bind (current-scope) [] 123) _ :mismatch)`)

	Ground.Set("meta",
		Func("meta", "[val]", func(val Value) Value {
			var ann Annotated
			if err := val.Decode(&ann); err == nil {
				return ann.Meta
			}

			return Null{}
		}),
		`returns the meta attached to the value`,
		`Returns null if the value has no metadata.`,
		`=> (meta meta) ; whoa`)

	Ground.Set("with-meta",
		Func("with-meta", "[val meta]", WithMeta),
		`returns val with the given scope as its metadata`,
		`=> (meta (with-meta _ {:a 1}))`,
		`=> (meta (with-meta (with-meta _ {:a 1}) {:b 2}))`)

	Ground.Set("doc",
		Op("doc", "symbols", PrintDocs),
		`print docs for symbols`,
		`Prints the documentation for the given symbols resolved from the current scope.`,
		`=> (doc doc)`)

	for _, pred := range primPreds {
		Ground.Set(pred.name, Func(string(pred.name), "[val]", pred.check), pred.docs...)
	}

	Ground.Set("+",
		Func("+", "nums", func(nums ...int) int {
			sum := 0
			for _, num := range nums {
				sum += num
			}

			return sum
		}),
		`sums numbers`,
		`=> (+ 1 2 3)`)

	Ground.Set("*",
		Func("*", "nums", func(nums ...int) int {
			mul := 1
			for _, num := range nums {
				mul *= num
			}

			return mul
		}),
		`multiplies numbers`,
		`=> (* 2 3 7)`)

	Ground.Set("quot",
		Func("quot", "[num denom]", func(num, denom int) int {
			return num / denom
		}),
		`quot(ient) of dividing num by denum`,
		`=> (quot 84 2)`)

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
		`subtracts ys from x`,
		`If only x is given, returns the negation of x.`,
		`=> (- 10 4)`,
		`=> (- 10 4 1)`,
		`=> (- 6)`)

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
		`returns the largest number`,
		`=> (max 6 42 7)`)

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
		`returns the smallest number`,
		`=> (min 6 42 7)`)

	Ground.Set("=",
		Func("=", "[val & vals]", func(val Value, others ...Value) bool {
			for _, other := range others {
				if !other.Equal(val) {
					return false
				}
			}

			return true
		}),
		`returns true if the values are all equal`,
		`=> (= 1 1 1 1)`,
		`=> (= :hello :hello :goodbye)`,
		`=> (= {:a 1} {:a 1})`,
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
		`returns true if the numbers are in descending order`,
		`=> (> 9 8 7)`,
		`=> (> 9 8 8)`)

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
		`returns true if the numbers are in descending or equal order`,
		`=> (> 9 8 7)`,
		`=> (> 9 8 8)`)

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
		`returns true if the numbers are in ascending order`,
		`=> (< 7 8 9)`,
		`=> (> 8 8 9)`)

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
		`returns true if the numbers are in ascending or equal order`,
		`=> (< 7 8 9)`,
		`=> (> 8 8 9)`)

	Ground.Set("list->source",
		Func("list->source", "[list]", func(list []Value) Value {
			return &Source{NewInMemorySource(list...)}
		}),
		"creates a pipe source from a list of values in chronological order",
		`=> (list->source [1 2 3])`)

	Ground.Set("across",
		Func("across", "sources", Across),
		"returns a pipe source that yields a list of values across all the given sources",
		`Each list has the last value for each source. Values from each source are never skipped, but not every combination will be produced.`,
		`=> (def evens (list->source [0 2 4]))`,
		`=> (def odds (list->source [1 3 5]))`,
		`=> (def combined (across evens odds))`,
		`=> [(next combined) (next combined)]`)

	Ground.Set("emit",
		Func("emit", "[val sink]", func(val Value, sink PipeSink) error {
			return sink.Emit(val)
		}),
		`emits a value to a sink`,
		`=> (emit {:a 1} *stdout*)`)

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
		`If the source has ended, no value will be available. A default value may be provided, otherwise an error is raised.`,
		`=> (next (list->source [1]) :eof)`,
		`=> (next *stdin* :eof)`)

	Ground.Set("reduce-kv",
		Wrap(Op("reduce-kv", "[f init kv]", func(ctx context.Context, scope *Scope, fn Applicative, init Value, kv *Scope) (Value, error) {
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
		})),
		`reduces a scope`,
		`Takes a 3-arity function, an initial value, and a scope. If the scope is empty, the initial value is returned. Otherwise, calls the function for each key-value pair, with the current value as the first argument.`,
		`=> (reduce-kv assoc {:d 4} {:a 1 :b 2 :c 3})`,
	)

	Ground.Set("assoc",
		Func("assoc", "[obj & kvs]", Assoc),
		`assoc(iate) keys with values in a clone of a scope`,
		`Takes a scope and a flat pair sequence alternating symbols and values.`,
		`Returns a clone of the scope with the symbols fields set to their associated value.`,
		`=> (assoc {:a 1} :b 2 :c 3)`,
	)

	Ground.Set("symbol->string",
		Func("symbol->string", "[sym]", func(sym Symbol) String {
			return String(sym)
		}),
		`convert a symbol to a string`,
		`=> (symbol->string :hello!)`)

	Ground.Set("string->symbol",
		Func("string->symbol", "[str]", func(str String) Symbol {
			return Symbol(str)
		}),
		`convert a string to a symbol`,
		`=> (string->symbol "hello!")`)

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
		`returns the concatenation of all given strings or values`,
		`=> (str "abc" 123 "def" 456)`)

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
		`With two numbers supplied, returns the portion between the first offset and the last offset, exclusive.`,
		`=> (substring "abcdef" 2 4)`)

	Ground.Set("trim",
		Func("trim", "[str]", strings.TrimSpace),
		`removes whitespace from both ends of a string`,
		`=> (trim " hello world!\n ")`)

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
		`The returned list is the same form accepted by [assoc].`,
		`=> (scope->list {:a 1 :b 2 :c 3})`,
		`=> (apply assoc (cons {:d 4} (scope->list {:a 1 :b 2 :c 3})))`)

	Ground.Set("string->fs-path",
		Func("string->fspath", "[str]", func(s string) FilesystemPath {
			return ParseFileOrDirPath(s).FilesystemPath()
		}),
		`parses a string value into a file or directory path`,
		`=> (string->fs-path "./file")`,
		`=> (string->fs-path "file")`,
		`=> (string->fs-path "dir/")`)

	Ground.Set("string->cmd-path",
		Func("string->cmd-path", "[str]", func(s string) Path {
			if !strings.Contains(s, "/") {
				return CommandPath{s}
			}

			fod := ParseFileOrDirPath(s)

			if fod.Dir != nil {
				// convert
				return FilePath{
					Path: fod.Dir.Path,
				}
			}

			return *fod.File
		}),
		`converts a string to a command or file path`,
		`If the value contains a /, it is converted into a file path.`,
		`Otherwise, the given value is converted into a command path.`,
		`=> (string->cmd-path "scripts/foo")`,
		`=> (string->cmd-path "bash")`)

	Ground.Set("string->dir",
		Func("string->dir", "[str]", func(s string) DirPath {
			fod := ParseFileOrDirPath(s)

			if fod.File != nil {
				return DirPath{
					Path: fod.File.Path,
				}
			}

			return *fod.Dir
		}),
		`converts a string to a directory path`,
		`A trailing slash is not required; the path is always assumed to be a directory.`,
		`=> (string->dir "dir")`,
		`=> (string->dir "dir/")`)

	Ground.Set("subpath",
		Func("subpath", "[parent-dir child-path]", (Path).Extend),
		`extend path with another path`,
		`=> (subpath ./dir/ ./file)`,
		`=> (subpath (.tests) ./coverage.html)`)

	Ground.Set("path-name",
		Func("path-name", "[path]", (Path).Name),
		`returns the base name of the path`,
		`For a command path, this returns the command name.`,
		`For a file or dir path, it returns the file or dir name.`,
		`For a file path, it returns the file name.`,
		`For a thunk, it returns the thunk's hash.`,
		`=> (path-name .bash)`,
		`=> (path-name ./some/file)`,
		`=> (path-name ./some/dir/)`,
		`=> (path-name (.tests))`,
	)

	Ground.Set("path-stem",
		Func("path-stem", "[path]", func(p Path) string {
			name := p.Name()

			ext := path.Ext(name)
			if ext != "" {
				name = name[:len(name)-len(ext)]
			}

			return name
		}),
		`returns the base name of the path, without any extension`,
		`=> (path-stem .bash)`,
		`=> (path-stem ./some/file.bass)`,
		`=> (path-stem ./some/dir/)`,
		`=> (path-stem (.tests))`,
	)

	// thunk constructors
	Ground.Set("with-image",
		Func("with-image", "[thunk image]", (Thunk).WithImage),
		`returns thunk with the base image set to image`,
		`Image is either a thunk? or an image ref.`,
		`Recurses when thunk's image is another thunk, setting the deepest ref or unset image.`,
		`See also [from].`,
		`=> (with-image ($ go test ./...) (linux/golang))`,
		`=> (from (linux/golang) ($ go test ./...))`)

	Ground.Set("with-dir",
		Func("with-dir", "[thunk dir]", (Thunk).WithDir),
		`returns thunk with the working directory set to dir`,
		`Unlike [cd], the value of [with-dir] is resolved at runtime, meaning it can use container-local paths.`,
		`If the thunk needs to write to its output directory, the output path passed to the command must be relative to the given dir. Thunk paths and other mounts will always be 1 level deep in the output directory, so use ../ to refer to back to the output directory, repeated for each additional level of depth. If the depth is unknown, you should use [cd] instead.`,
		`=> (with-dir (.tests) ./src/)`)

	Ground.Set("with-args",
		Func("with-args", "[thunk args]", (Thunk).WithArgs),
		`returns thunk with args set to args`,
		`=> (with-args (.go) ["test" "./..."])`)

	Ground.Set("with-cmd",
		Func("with-cmd", "[thunk cmd]", (Thunk).WithCmd),
		`returns thunk with cmd set to cmd`,
		`=> (let [inner (with-args (.go) ["build"])] (with-args (with-cmd inner ./wrapped) (cons (thunk-cmd inner) (thunk-args inner))))`)

	Ground.Set("with-stdin",
		Func("with-stdin", "[thunk vals]", (Thunk).WithStdin),
		`returns thunk with stdin set to vals`,
		`=> (with-stdin ($ jq ".a") [{:a 1} {:a 2}])`)

	Ground.Set("with-env",
		Func("with-env", "[thunk env]", (Thunk).WithEnv),
		`returns thunk with env set to the given env`,
		`=> (with-env ($ jq ".a") {:FOO "hello"})`)

	Ground.Set("with-insecure",
		Func("with-insecure", "[thunk bool]", (Thunk).WithInsecure),
		`returns thunk with the insecure flag set to bool`,
		`The insecure flag determines whether the thunk runs with elevated privileges, and is named to be indicate the reduced security assumptions.`,
		`=> (with-insecure (.boom) true)`,
		`=> (= (.boom) (with-insecure (.boom) false))`)

	Ground.Set("with-label",
		Func("with-label", "[thunk name val]", (Thunk).WithLabel),
		`returns thunk with the label set to val`,
		`Labels are typically used to control caching. Two thunks that differ only in labels will evaluate separately and produce independent results.`,
		`=> (with-label ($ sleep 10) :at (now 10))`)

	Ground.Set("with-port",
		Func("with-port", "[thunk sym int]", (Thunk).WithPort),
		`returns thunk with a named port appended to its ports`,
		`=> (with-port ($ godoc "-http=:6060") :godoc 6060)`)

	Ground.Set("with-tls",
		Func("with-tls", "[thunk cert-path key-path]", (Thunk).WithTLS),
		`returns thunk with paths to a TLS certificate and key to generate`,
		`=> (with-tls ($ godoc "-http=:6060") ./cert.pem ./key.pem)`)

	Ground.Set("with-mount",
		Func("with-mount", "[thunk source target]", (Thunk).WithMount),
		`returns thunk with a mount from source to the target path`,
		`=> (with-mount ($ find ./inputs/) *dir*/inputs/ ./inputs/)`)

	Ground.Set("thunk-cmd",
		Func("thunk-cmd", "[thunk]", func(thunk Thunk) Value {
			return thunk.Cmd.ToValue()
		}),
		`returns the thunk's command`,
		`=> (thunk-cmd (.foo))`,
		`=> (thunk-cmd (./foo))`)

	Ground.Set("thunk-args",
		Func("thunk-args", "[thunk]", func(thunk Thunk) Value {
			return NewList(thunk.Args...)
		}),
		`returns the thunk's args`,
		`=> (thunk-args ($ foo abc))`,
		`=> (thunk-args ($ foo))`)

	Ground.Set("load",
		Func("load", "[thunk]", Bass.Load),
		`load a thunk as a module`,
		`This is the primitive mechanism for loading other Bass code.`,
		`Typically used in combination with *dir* to load paths relative to the current file's directory.`,
		`=> (load (.strings))`)

	Ground.Set("resolve",
		Func("resolve", "[platform ref]", func(ctx context.Context, ref ImageRef) (ImageRef, error) {
			runtime, err := RuntimeFromContext(ctx, ref.Platform)
			if err != nil {
				return ImageRef{}, err
			}

			return runtime.Resolve(ctx, ref)
		}),
		`resolve an image reference to its most exact form`,
		`=> (resolve {:platform {:os "linux"} :repository "golang" :tag "latest"})`)

	Ground.Set("start",
		Func("start", "[thunk handler]", func(ctx context.Context, thunk Thunk, handler Combiner) (Combiner, error) {
			return thunk.Start(ctx, handler)
		}),
		`starts running a thunk asynchronously`,
		`If the thunk errors or exits nonzero the handler is called with a combiner that raises the error when called.`,
		`If the thunk runs succeeds the handler is called with null.`,
		`=> (start (from (linux/alpine) ($ banana)) null?)`,
		`=> ((start (from (linux/alpine) ($ banana)) null?))`,
		`=> ((start (from (linux/alpine) ($ echo)) null?))`,
		`=> (defn raiser [err] (and err (err)))`,
		`=> ((start (from (linux/alpine) ($ banana)) raiser))`,
		`=> ((start (from (linux/alpine) ($ echo)) raiser))`)

	Ground.Set("addr", Func("addr", "[thunk port & fmt]", (Thunk).Addr),
		`returns an address for a port provided by the thunk`,
		`Takes an optional format argument which defaults to "$host:$port".`,
		`=> (def thunk (-> ($ python -m http.server) (with-port :http 8080)))`,
		`=> (addr thunk :http)`)

	Ground.Set("wait",
		Func("wait", "[]", func(ctx context.Context) error {
			return RunsFromContext(ctx).Wait()
		}),
		`waits for all started thunks to finish`,
		`Returns an error if any of the thunk handlers error.`,
		`=> (defn echo-server [msg] (start (from (linux/alpine) ($ sleep 1 $msg)) null?))`,
		`=> (wait)`)

	Ground.Set("read",
		Func("read", "[thunk-or-file protocol]", func(ctx context.Context, read Readable, proto Symbol) (*Source, error) {
			rc, err := read.Open(ctx)
			if err != nil {
				return nil, err
			}

			if cust, ok := CustodianFrom(ctx); ok {
				cust.AddCloser(rc)
			}

			src, err := DecodeProto(ctx, proto, rc)
			if err != nil {
				return nil, err
			}

			return NewSource(src), nil
		}),
		`returns a stream producing values read from a thunk's output or a file's content`,
		`=> (def echo-thunk (from (linux/alpine) ($ echo "42")))`,
		`=> (next (read echo-thunk :json))`,
		`=> (def file-thunk (from (linux/alpine) ($ sh -c "echo 42 > file")))`,
		`=> (next (read file-thunk/file :json))`,
	)

	Ground.Set("cache-dir",
		Func("cache-dir", "[id]", NewCacheDir),
		`returns a cache directory corresponding to the string identifier`,
		`Cache directories may be mounted to thunks. Their content persists across thunk runs.`)

	Ground.Set("binds?",
		Func("binds?", "[scope sym]", (*Scope).Binds),
		`returns true if the scope has a value bound to the given symbol`,
		`=> (binds? {:x 1} :x)`,
		`=> (binds? {} :x)`,
		`=> (binds? (current-scope) :binds?)`)

	Ground.Set("write",
		Func("write", "[src dest]", func(ctx context.Context, src Readable, dest Writable) error {
			r, err := src.Open(ctx)
			if err != nil {
				return err
			}

			defer r.Close()

			return dest.Write(ctx, r)
		}),
		`writes the source to the destination path`,
		`Source is any value that can be passed to (read), i.e. a thunk or a path.`,
		`Writes are atomic. The content will first be written to dest.new and renamed to dest.`,
		`=> (write (from (linux/alpine) ($ echo "Hello, world!")) *dir*/hello)`)

	Ground.Set("publish",
		Func("publish", "[src ref]", func(ctx context.Context, thunk Thunk, ref string) (ImageRef, error) {
			r, err := reference.ParseDockerRef(ref)
			if err != nil {
				return ImageRef{}, err
			}

			nt, ok := r.(reference.NamedTagged)
			if !ok {
				return ImageRef{}, fmt.Errorf("ref must be named and tagged, have %T: %s", r, ref)
			}

			return thunk.Publish(ctx, ImageRef{
				Repository: ImageRepository{Static: nt.Name()},
				Tag:        nt.Tag(),
			})
		}),
		`publishes the thunk to a container registry`,
		`Returns a fully qualified image reference.`,
		`=> (publish (from (linux/golang) ($ go version)) "basslang/publish-demo")`)
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
	{"null?", func(val Value) bool {
		var x Null
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is null`,
		`=> (null? null)`,
		`=> (null? _)`,
		`=> (null? false)`,
	}},

	{"ignore?", func(val Value) bool {
		var x Ignore
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is _ ("ignore")`,
		`=> (ignore? _)`,
		`=> (ignore? null)`,
	}},

	{"boolean?", func(val Value) bool {
		var x Bool
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is true or false`,
		`=> (boolean? null)`,
		`=> (boolean? true)`,
		`=> (boolean? false)`,
	}},

	{"number?", func(val Value) bool {
		var x Int
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a number`,
		`=> (number? 123)`,
		`=> (number? "123")`,
	}},

	{"string?", func(val Value) bool {
		var x String
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a string`,
		`=> (string? "abc")`,
		`=> (string? :abc)`,
	}},

	{"symbol?", func(val Value) bool {
		var x Symbol
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a symbol`,
		`=> (symbol? :abc)`,
		`=> (symbol? "abc")`,
	}},

	{"scope?", func(val Value) bool {
		var x *Scope
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a scope`,
		`A scope is a mapping from symbols to values.`,
		`=> (scope? {})`,
		`=> (scope? (current-scope))`,
		`=> (scope? [])`,
	}},

	{"sink?", func(val Value) bool {
		var x *Sink
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a sink`,
		`A sink is a type that you can send values to using [emit].`,
		`=> (sink? *stdout*)`,
		`=> (sink? *stdin*)`,
	}},

	{"source?", func(val Value) bool {
		var x *Source
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a source`,
		`A source is a type that you can read values from using [next].`,
		`=> (source? *stdin*)`,
		`=> (source? *stdout*)`,
	}},

	{"list?", IsList, []string{
		`returns true if the value is a linked list`,
		`A linked list is a pair whose second value is another list or empty.`,
		`=> (list? [])`,
		`=> (list? {})`,
	}},

	{"pair?", func(val Value) bool {
		var x Pair
		if val.Decode(&x) == nil {
			return true
		}

		var c Cons
		if val.Decode(&c) == nil {
			return true
		}

		return false
	}, []string{
		`returns true if the value is a pair`,
		`=> (pair? [])`,
		`=> (pair? [1])`,
		`=> (pair? [1 & 2])`,
		`=> (pair? (quote [1 & 2]))`,
	}},

	{"applicative?", IsApplicative, []string{
		`returns true if the value is an applicative`,
		`An applicative is a combiner that wraps another combiner.`,
		`When an applicative is called, it evaluates its operands in the caller's evironment and passes them to the underlying combiner.`,
		`=> (applicative? applicative?)`,
		`=> (applicative? op)`,
	}},

	{"operative?", IsOperative, []string{
		`returns true if the value is an operative`,
		`An operative is a combiner that is given the caller's scope.`,
		`An operative may decide whether and how to evaluate its arguments. They are typically used to define new syntactic constructs.`,
		`=> (operative? applicative?)`,
		`=> (operative? op)`,
	}},

	{"combiner?", func(val Value) bool {
		var x Combiner
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a combiner`,
		`A combiner takes sequence of values as arguments and returns another value.`,
		`=> (combiner? applicative?)`,
		`=> (combiner? op)`,
	}},

	{"path?", func(val Value) bool {
		var x Path
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a path`,
		`A path is a reference to a file, directory, or command.`,
		`=> (path? ./foo)`,
		`=> (path? .foo)`,
		`=> (path? (subpath (.tests) ./coverage.html))`,
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
		`=> (empty? [])`,
		`=> (empty? "")`,
		`=> (empty? {})`,
		`=> (empty? null)`,
		`=> (empty? :my-soul)`,
	}},

	{"thunk?", func(val Value) bool {
		var x Thunk
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a valid thunk`,
		`=> (thunk? (.yep))`,
		`=> (thunk? [.nope])`,
		`=> (thunk? {:not-even "close"})`,
	}},
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

func zapField(k Symbol, v Value) (zap.Field, error) {
	name := k.String()

	var str string
	var num int
	var bol bool
	var am zapcore.ArrayMarshaler
	var om zapcore.ObjectMarshaler
	if v.Decode(&str) == nil {
		return zap.String(name, str), nil
	} else if v.Decode(&num) == nil {
		return zap.Int(name, num), nil
	} else if v.Decode(&bol) == nil {
		return zap.Bool(name, bol), nil
	} else if v.Decode(&am) == nil {
		return zap.Array(name, am), nil
	} else if v.Decode(&om) == nil {
		return zap.Object(name, om), nil
	}

	return zap.Field{}, EncodeError{v}
}
