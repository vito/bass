# bass

[![Discord](https://img.shields.io/discord/939941216215240774?color=7389D8&label=&logo=discord&logoColor=ffffff&labelColor=6A7EC2)](https://discord.gg/HFW85RyUtK)

Bass is a low-fidelity Lisp dialect for the glue code driving your project.

https://github.com/vito/bass/assets/1880/ab05445c-95f7-44b6-a67b-d9fc8eb02d41

## reasons you might be interested

* you're sick of YAML and want to write code instead of config and templates
* you'd like to have a uniform stack between dev and CI
* you'd like be able to audit or rebuild published artifacts
* you're nostalgic about Lisp

## what the thunk?

Bass is a functional language for scripting commands, represented by
[thunks][t-thunk]. A thunk is a serializable recipe for a command to run,
including all of its inputs, and their inputs, and so on. ([Why are they called
thunks?][why-thunks])

Thunks lazily run their command to produce a `stdout` stream, an output
directory, and an exit status. These results are cached indefinitely, but only
when the command succeeds.

```clojure
$ bass
=> (from (linux/alpine) ($ cat *dir*/README.md))


        ██████
      ██████████
    ██████████████
    ████  ██  ████
    ██████████████
    ██████  ██████
  ████    ██    ████
    ████      ████

<thunk JU61UMJQ70FMI: (.cat)>
=> (thunk? (from (linux/alpine) ($ cat *dir*/README.md)))
true
```

To run a thunk and raise an error if the command fails, use [`(run)`][b-run].
To get `true` or `false` instead, use [`(succeeds?)`][b-succeeds].

```clojure
=> (def thunk (from (linux/alpine) ($ echo "Hello, world!")))
thunk
=> (run thunk)
; Hello, world!
null
=> (succeeds? thunk)
true
=> (succeeds? (from (linux/alpine) ($ sh -c "exit 1")))
false
```

To access a thunk's output directory, use a [thunk path][t-thunk-path]. Thunk
paths can be passed to other thunks. Filesystem timestamps in thunk paths are
normalized to `1985-10-06 08:15 UTC` to support reproducible builds.

```clojure
=> (def thunk (from (linux/alpine) ($ cp *dir*/README.md ./some-file)))
thunk
=> (run (from (linux/alpine) ($ head "-1" thunk/some-file)))
; # bass
null
```

To parse values from a thunk's `stdout` or from a thunk path, use
[`(read)`][b-read].

```clojure
=> (next (read (from (linux/alpine) ($ head "-1" thunk/some-file)) :raw))
"# bass\n"
=> (next (read thunk/some-file :lines))
"# bass"
=> (next (read thunk/some-file :unix-table))
("#" "bass")
```

To serialize a thunk or thunk path to JSON, use [`(json)`][b-json] or
[`(emit)`][b-emit] it to `*stdout*`. Pipe a thunk path to `bass --export | tar
-xf -` to extract it, or pipe a thunk to `bass --export | docker load` to
export a thunk to Docker.

```sh
$ ./bass/build -i src=./ | bass --export | tar -xf -
$ ls bass.linux-amd64.tgz
```

This, and generally everything, works best when your thunks are
[hermetic][t-hermetic].

#### tl;dr

It's a bit of a leap, but I like to think of Bass as a purely functional,
lazily evaluated Bash.

Instead of running commands that mutate machine state, Bass has a read-only
view of the host machine and passes files around as values in ephemeral,
reproducible filesystems addressed by their creator thunk.

[b-read]: https://bass-lang.org/stdlib.html#binding-read
[b-run]: https://bass-lang.org/stdlib.html#binding-run
[b-succeeds]: https://bass-lang.org/stdlib.html#binding-succeeds?
[b-json]: https://bass-lang.org/stdlib.html#binding-json
[b-emit]: https://bass-lang.org/stdlib.html#binding-emit

## example

Running a [thunk][t-thunk]:

```clojure
(def thunk
  (from (linux/ubuntu)
    ($ echo "Hello, world!")))

(run thunk)
```

Passing [thunk paths][t-thunk-path] around:

```clojure
; use git stdlib module
(use (.git (linux/alpine/git)))

; returns a thunk dir containing compiled binaries
(defn go-build [src pkg]
  (subpath
    (from (linux/golang)
      (cd src
        ($ go build -o ./built/ $pkg)))
    ./built/))

(defn main []
  (let [src git:github/vito/booklit/ref/master/
        bins (go-build src "./cmd/...")]
    ; kick the tires
    (run (from (linux/ubuntu)
           ($ bins/booklit --version)))

    (emit bins *stdout*)))
```

### irl examples

* [Bass](bass/)
* [Booklit](https://github.com/vito/booklit/tree/master/bass)

## what's it for?

Bass typically replaces CI `.yml` files, `Dockerfile`s, and Bash scripts.

Instead of writing `.yml` DSLs interpreted by some CI system, you write real
code. Instead of writing ad-hoc `Dockerfile`s and pushing/pulling images, you
chain thunks and share them as code. Instead of writing Bash scripts, you write
Bass scripts.

Bass scripts have limited access to the host machine, making them portable
between dev and CI environments. They can be used to codify your entire
toolchain into platform-agnostic scripts.

In the end, the purpose of Bass is to run [thunks][t-thunk]. Thunks are
serializable command recipes that produce files or streams of values. Files
created by thunks can be easily passed to other thunks, forming one big
super-thunk that recursively embeds all of its dependencies.

Bass is designed for [hermetic][t-hermetic] builds but it stops short of
enforcing them. Bass trades purism for pragmatism, sticking to familiar albeit
fallible CLIs rather than abstract declarative configuration. For your
artifacts to be reproducible your thunks must be hermetic, but if you simply
don't care yet, YOLO `apt-get` all day and fix it up later.

For a quick run-through of these ideas, check out the [Bass homepage][bass].

[bass]: https://bass-lang.org
[llb]: https://github.com/moby/buildkit/blob/master/docs/solver.md
[t-thunk]: https://bass-lang.org/bassics.html#term-thunk
[t-thunk-path]: https://bass-lang.org/bassics.html#term-thunk%20path
[t-hermetic]: https://bass-lang.org/bassics.html#term-hermetic

### how does it work?

To run a thunk, Bass's [Buildkit][buildkit] runtime translates it to one big
[LLB][llb] definition and solves it through the client API. The runtime handles
setting up mounts and converting thunk paths to string values passed to the
underlying command.

The runtime architecture is modular, but Buildkit is the only implementation at
the moment.


## start playing

* prerequisites: `git`, `go`, `upx`

```sh
$ git clone https://github.com/vito/bass
$ cd bass
$ make -j install
```

Bass runs thunks with [Buildkit][buildkit-quickstart], so you'll need
`buildkitd` running somewhere, somehow.

If `docker` is installed and running Bass will use it to start Buildkit
automatically and you can skip the rest of this section.

### Linux

The included `./hack/buildkit/` scripts can be used if you don't have
`buildkitd` running already.

```sh
$ ./hack/buildkit/start # if needed
$ bass ./demos/go-build-git.bass
```

[buildkit-quickstart]: https://github.com/moby/buildkit#quick-start

### macOS

macOS support works by just running Buildkit in a Linux VM.

Use the included [`lima/bass.yaml`](lima/bass.yaml) template to manage the VM
with [`limactl`][lima].

```sh
$ brew install lima
$ limactl start ./lima/bass.yaml
$ bass ./demos/go-build-git.bass
```

[lima]: https://github.com/lima-vm/lima

### Windows

Same as Linux, using WSL2. Windows containers should work [once Buildkit
supports it][windows].

[windows]: https://github.com/moby/buildkit/issues/616

## editor setup

* vim config: [bass.vim][bass.vim]

[bass.vim]: https://github.com/vito/bass.vim

```vim
Plug 'vito/bass.vim'

lua <<EOF
require'bass_lsp'.setup()
EOF
```

## cleaning up

The Buildkit runtime leaves snapshots around for caching thunks, so if you
start to run low on disk space you can run the following to clear them:

```
$ bass --prune
```


## the name

Bass is named after the :loud_sound:, not the :fish:. Please do not think of
the :fish: every time. It will eventually destroy me.


## rationale

### motivation

After 6 years working on [Concourse][concourse] I felt pretty unsatisfied and
burnt out. I wanted to solve CI/CD "once and for all" but ended up being
overwhelmed with complicated problems that distracted from the core goal:
database migrations, NP hard visualizations, scalability, resiliency, etc. etc.
etc.

When it came to adding core features, it felt like building a language confined
to a declarative YAML schema and driven by a complex state machine. So I wanted
to try just building a damn language instead, since that's what I had fun with
back in the day ([Atomy][atomy], [Atomo][atomo], [Hummus][hummus],
[Cletus][cletus], [Pumice][pumice]).

### why a new Lisp?

I think the pattern of YAML DSLs interpreted by DevOps services may be evidence
of a gap in our toolchain that could be filled by something more expressive.
I'm trying to discover a language that fills that gap while being small enough
to coexist with all the other crap a DevOps engineer has to keep in their head.

After writing enterprise cloud software for so long, it feels good to return to
the loving embrace of `(((one thousand parentheses)))`. For me, Lisp is the
most fun you can have with programming. Lisps are also known for doing a lot
with a little - which is exactly what I need for this project.

### Kernel's influence

Bass is a descendant of the [Kernel programming language][kernel]. Kernel is
the tiniest Lisp dialect I know of - it has a primitive form _beneath_
`lambda` called `$vau` ([`op`][b-op] in Bass) which it leverages to replace the
macro system found in most other Lisp dialects.

Unfortunately this same mechanism makes Kernel difficult to optimize for
production applications, but Bass targets a domain where its own performance
won't be the bottleneck, so it seems like a good opportunity to share Kernel's
ideas with the world.

[b-op]: https://bass-lang.org/stdlib.html#binding-op

### Clojure's influence

Bass marries Kernel's semantics with Clojure's vocabulary and ergonomics,
because you should never have to tell a coworker that the function to get the
first element of a list is called :car:. A practical Lisp should be accessible
to engineers from any background.

[arrow]: https://bass-lang.org/stdlib.html#binding--%3e
[t-operative]: https://bass-lang.org/bassics.html#term-operative


## is it any good?

It's pretty close.

I'm using it for my projects and enjoying it so far, but there are still some
limitations and rough edges.


## project expectations

This project is built for fun and is developed in my free time. I'm just trying
to build something that I would want to use for my own projects. I don't plan
to bear the burden of large enterprises using it, but I'm interested in
collaborating with and supporting hobbyists.


## how can I help?

Try it out! I'd love to hear [experience reports][discussions] especially if
things don't go well. This project is still young, and it only gets better the
more it gets used.

Pull requests are very welcome, but this is still a personal hobby so I will
probably push back on contributions that substantially increase the maintenance
burden or technical debt (...unless they're wicked cool).

For more guidance, see the [contributing docs](CONTRIBUTING.md).

[discussions]: https://github.com/vito/bass/discussions


## thanks

* John Shutt, creator of the [Kernel] programming language.
* Rich Hickey, creator of the [Clojure] programming language.
* The [Buildkit project][buildkit], which powers the default runtime.
* [MacStadium], who have graciously donated hardware for testing macOS support.

<img alt="MacStadium logo" src="https://uploads-ssl.webflow.com/5ac3c046c82724970fc60918/5c019d917bba312af7553b49_MacStadium-developerlogo.png" width="200" />

[MacStadium]: https://www.macstadium.com/
[kernel]: https://web.cs.wpi.edu/~jshutt/kernel.html
[clojure]: https://clojure.org/

[go]: https://golang.org
[concourse]: https://github.com/concourse/concourse
[oci]: https://github.com/opencontainers/image-spec
[atomo]: https://github.com/vito/atomo
[atomy]: https://github.com/vito/atomy
[pumice]: https://github.com/vito/pumice
[cletus]: https://github.com/vito/cletus
[hummus]: https://github.com/vito/hummus
[resources]: https://concourse-ci.org/resources.html
[tasks]: https://concourse-ci.org/tasks.html
[jq]: https://stedolan.github.io/jq/
[concourse-types]: https://resource-types.concourse-ci.org/
[json]: https://www.json.org/
[streams]: https://en.wikipedia.org/wiki/Standard_streams
[buildkit]: https://github.com/moby/buildkit
[booklit-test]: https://github.com/vito/booklit/blob/master/ci/test.yml
[booklit-build]: https://github.com/vito/booklit/blob/master/ci/build.yml
[why-thunks]: https://github.com/vito/bass-loop/discussions/4
