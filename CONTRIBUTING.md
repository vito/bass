# Contributing to Bass

Hi there! This guide covers the various ways you can help Bass get better,
whether that's writing code or just providing good signals to the project.


## Contributing feedback & discussions

Languages evolve as more people speak them. Bass is very young, so feedback is
incredibly valuable. Using it for my own projects has led to dramatic changes,
and it'd be best to get the bulk of these out of the way early on.

The best place to leave feedback is in [Discussions], but feel free to just hop
in [Discord] too.

It's hard to use a language without having something to say, so if you don't
have a project to apply Bass to feel free to critique Bass's own Bass code:

* [project.bass](project.bass) contains the bulk of the project code.
* [ci/shipit](ci/shipit) is the script for building shipping Bass versions.
* [hack/build](hack/build) builds Bass.
* [hack/build-docs](hack/build-docs) builds Bass's docs.
* [hack/test](hack/test) runs Bass's test suite.

[discussions]: https://github.com/vito/bass/discussions
[discord]: https://discord.gg/HFW85RyUtK


## Writing code

Bass is written in Go. Most of the code lives under [pkg/](pkg/) and
[cmd/](cmd/).

To build Bass from source, run:

```sh
make -j install
```

This command builds the `bass` executable along with its embedded runtime
shims, needed for running containers in the Buildkit runtime. The shim doesn't
change often, so you can go back to `go install` muscle memory after this
point.


### Running & testing your changes

Please write tests. :) I will admit I've taken shortcuts here and there, but
this is a debt that I won't let grow out of control.

To run the tests:

```sh
go test ./...
```

The tests assume Buildkit is running somewhere, and they discover it the same
way `bass` does. See [Getting Started][getting-started] if you need to set this
up.

[getting-started]: https://bass-lang.org/guide.html#getting-started


### Source code primer

* [cmd/bass/](cmd/bass/) contains the `bass` CLI.

* [pkg/bass/](pkg/bass/) contains all core Bass language constructs.

* [pkg/bass/value.go](pkg/bass/value.go) defines the `Value` interface that all
  Bass values implement.

* [pkg/bass/ground.go](pkg/bass/ground.go) defines the [`Ground`
  module][ground-docs] inherited by all Bass modules.

* [pkg/bass/memo.go](pkg/bass/memo.go) extends `Ground` with `bass.lock`
  mechanisms.

* [pkg/bass/eval.go](pkg/bass/eval.go) provides high-level interfaces for
  evaluating Bass source code, plus a [Trampoline](#continuation-passing-style).

* [pkg/runtimes/](pkg/runtimes/) contains the runtimes for running thunks.

* [pkg/runtimes/buildkit.go](pkg/runtimes/buildkit.go) defines the Buildkit
  runtime, used for running commands in containers.

* [pkg/runtimes/bass.go](pkg/runtimes/bass.go) defines the Bass runtime, used
  for loading Bass modules or running Bass scripts.

* [pkg/runtimes/suite.go](pkg/runtimes/suite.go) defines the unified Bass
  runtime test suite, which all runtime implementations must be able to pass,
  except for the Bass runtime which has its own separate suite.

* [pkg/lsp/](pkg/lsp/) defines the language server behind `bass --lsp`.

[value-code]: https://github.com/vito/bass/blob/main/pkg/bass/value.go
[ground-docs]: https://bass-lang.org/stdlib.html#ground


### Continuation-passing style

A quick sidebar: to support tail recursion, Bass's Go implementation is written
in [continuation-passing style][cps].

Instead of simply evaluating a Bass form and getting a result (direct style),
the form is passed a *continuation* which it calls it with its result,
returning a deferred continuation. An outer loop called a
[trampoline][trampoline] continuously calls the deferred continuation and stops
when an inert value is reached.

This results in pretty funky function signatures which force you to jump
through hoops to transform direct-style code to continuation-passing style. But
it's worth it!

```go
// no: direct style
func (Foo) Eval(context.Context, *Scope) (Value, error) {
  return Null{}, nil
}

// yes: continuation-passing style
func (Foo) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
  return cont.Call(Null{}, nil)
}
```

The `ReadyCont` return value is a `Value` representing a deferred continuation
call which must be forced by calling `Go()`:

```go
type ReadyCont interface {
	Value

	Go() (Value, error)
}
```

The `Value` returned by `Go()` might itself be another `ReadyCont`. To keep
`Go`ing until you reach a non-`ReadyCont` value, pass it to `Trampoline`.

```go
func Trampoline(ctx context.Context, val Value) (Value, error)
```

This technique allows Bass to implement infinite loops without blowing up the
stack. It also technically means Bass supports `call/cc`, but I haven't found a
reason to expose that yet. It's a little spooky.

In general you should avoid calling `Trampoline` unless you really know what
you're doing. There should really only be one of these at the very top of the
call stack. Using it in more places will result in broken backtraces.

[cps]: https://en.wikipedia.org/wiki/Continuation-passing_style
[trampoline]: https://en.wikipedia.org/wiki/Tail_call#Through_trampolining


## Writing docs

You don't have to document every change, but I'd certainly appreciate it!

The content lives under [docs/lit/](docs/lit/), and it's written in
[Booklit][booklit] - a little tool for technical content authoring, kind of
like Racket's [Scribble][scribble].

Booklit is another hobby project of mine, so you're probably not familiar with
it, but hopefully it's not too hard to figure out.

To run the docs website on a local port:

```sh
./docs/scripts/booklit -s 3000
```

Modify the content to your heart's content and refresh. It's a little slow,
since the docs run Bass code inline, and I haven't figured out how to cache it
properly yet.

If you need to change the CSS, edit [docs/less](docs/less/) and run `make`. To
automatically rebuild when files are dirty, try out [`harry`][harry].

[booklit]: https://booklit.page
[scribble]: https://docs.racket-lang.org/scribble/
[harry]: https://github.com/vito/harry


## Unfinished business & potential research

I've had a lot of fun building Bass, but there's still a lot of potential for
even more fun ahead. Below are just some ideas in no particular order that I'm
interested in but would really welcome help with, since - well, there's just a
lot.

If you're interested in any of this, don't hesitate to reach out - I'm happy to
help!

### Excellent errors

I really want Bass to have excellent, helpful error messages like [Elm][elm].
I've defined a basic `NiceError` interface for this, but haven't applied it to
most places yet. Bass will already suggest opening an issue for non-nice
errors, but feel free to help chip away at these too.

### Bass Loop

A long-running form of Bass for CI/CD would be neat. Detecting external
changes, running builds, fanning out to parallel jobs and back in to
automatically ship/deploy code, etc.

I have an implementation of Concourse-style `passed` constraints stashed away
somewhere, and infinite loops are supported via
[CPS](#continuation-passing-style), but I haven't put 2 + 2 together yet, and
there's some low hanging fruit that will have to be addressed (e.g. I bet the
progress UI will just collect vertexes forever).

### Concurrency

Despite being written in Go, Bass does not currently expose any concurrency
constructs. This might be easy to add, I just haven't needed it yet, and we
should figure out the best way to represent it in the language first.

There may be dragons here, i.e. making `Value`s thread-safe, but thankfully the
scope of that is probably small since mutation isn't a common thing in Bass
scripts (besides settings things in a `*Scope`).

### Bass Compose

Using Bass to stand up local servers that talk to each other might be an
interesting alternative to Docker Compose. The tricky thing here will be
networking in Buildkit.

Some starting points:

* buildkit issue tracking dependent containers: [moby/buildkit#1337]
* failing that, thunks could be configured to use the [host network]

[host network]: https://github.com/moby/buildkit/blob/9ff8e772303bd3737971068ff1ab7770afd8dd58/solver/pb/ops.proto#L81
[moby/buildkit#1337]: https://github.com/moby/buildkit/issues/1337

### Clustered runtimes

Running thunks across a Nomad or Kubernetes cluster would be really neat. I
think this should be something driven by real need though, so I'm not working
on it myself.

### Supporting `(load)` with command thunks

Despite both being runtimes, the Bass runtime and Buildkit runtime aren't
symmetrical. The Buildkit runtime doesn't support `(load)`, and it's not
completely clear what that should do, or why the runtimes like Buildkit should
be expected to implement it.

I think there's potential for exposing module-like interfaces from OCI images,
but I haven't proposed anything concerete here yet, since I'm not sure if it'd
even be useful.

### Alternative Bass implementations

I'd be really interested in seeing alternative implementations of Bass. I chose
Go because I'm most familiar with it, and it has close proximity to the
containerization landscape. But I'm not married to this implementation.

### Alternative languages

I chose to make Bass a Lisp because it's familiar for me and easy to implement,
but it'd be interesting to apply Bass's concepts to different language
paradigms. In particular, static typing would probably be helpful.

### Interactive thunk building

Interactive building of thunks to generate Bass thunk code by running commands
in a (pseudo?) shell. Sometimes you just want to hop into an image and muck
around, and it'd be nice to not have to reach for `docker`, and even nicer to
be able to convert what you ran into code.

### Nix... something

I'm a fan of Nix, but from afar: I don't use it daily. If I did, I might not
have made Bass.

I'm interested in seeing whether Bass and Nix go together like peanut butter
and jelly, or if it's more like oil and water. I don't know Nix deeply enough
to tell.


[elm]: https://elm-lang.org/
