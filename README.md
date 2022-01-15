# bass

Bass is a low-fidelity Lisp dialect for scripting the infrastructure beneath
your project.

<p align="center">
  <img src="https://raw.githubusercontent.com/vito/bass/main/demos/readme.gif">
</p>


## reasons you might be interested

* you'd like to have a uniform stack between dev and CI
* you're sick of YAML and want to write code instead of config and templates
* you'd like be able to audit or rebuild published artifacts
* you're nostalgic about Lisp

## example

Bass is built for running thunks: containerized, cacheable commands.

```clojure
(def thunk
  (from (linux/ubuntu)
    ($ echo "Hello, world!")))

(run thunk)
```

Using CLIs as the primary building block sticks to what people know and helps
keep Bass small and focused.

```clojure
; use git stdlib module
(use (.git (linux/alpine/git)))

; returns a thunk dir containing compiled binaries
(defn go-build [src pkg]
  (let [thunk (from (linux/golang)
                 (cd src ($ go build -o ../built/ $pkg)))]
    thunk/built/))

(let [src git:github/vito/bass/ref/main/
      bins (go-build src "./cmd/...")]
  ; kick the tires
  (run (from (linux/ubuntu)
         ($ bins/bass --version)))

  (emit bins *stdout*))
```

The `(emit)` call writes a [thunk path][t-thunk-path] to `stdout` without
running it. It can be published (for provenance), archived (for auditing), or
built and extracted with `bass --export`:

```sh
$ ./example > binaries.json
$ cat binaries.json | bass --export | tar -xf -
```

For more demos, see [demos ; bass](https://bass-lang.org/demos.html).

### irl examples

* Bass: [ci/](ci/) and [hack/](hack/)
* [Booklit](https://github.com/vito/booklit/tree/master/ci) ([diff](https://github.com/vito/booklit/commit/cfa5e17dc5a7531e18245cae1c3501c99b1013b6))


## what's it for?

Bass can be used as an alternative to writing one-off Dockerfiles for setting
up CI dependencies. Bass scripts are isolated from the host machine, so they
can be re-used easily between dev and CI environments, or they can be used to
codify your entire toolchain in an open-source project.

In the end, the sole purpose of Bass is to run [thunks][t-thunk]. Thunks are
encodable data structures for running containerized commands that produce files
or return values. Files created by thunks can be easily passed to other thunks,
forming one big super-thunk that recursively embeds all of its dependencies.

Bass is designed for hermetic builds and provides a foundation for doing so,
but it stops short of enforcing it. It trades purity for ergonomics, sticking
to familiar CLIs instead of abstract declarative configs. For reproducible
artifacts, your thunks must be [hermetic][t-hermetic-thunk]. But if you simply
don't care yet, YOLO and `apt-get` all day and fix it up later.

For a quick run-through of these ideas, check out the [Bass homepage][bass].

[bass]: https://bass-lang.org
[llb]: https://github.com/moby/buildkit/blob/master/docs/solver.md
[t-thunk]: https://bass-lang.org/bassics.html#term-thunk
[t-thunk-path]: https://bass-lang.org/bassics.html#term-thunk%20path
[t-hermetic-thunk]: https://bass-lang.org/bassics.html#term-hermetic%20thunk

### how does it work?

To run a thunk, Bass's [Buildkit][buildkit] runtime translates it to one big
[LLB][llb] definition and solves it through the client API. The runtime handles
setting up mounts and converting thunk paths to string values passed to the
underlying command. The runtime architecture is modular, but Buildkit is the
only one so far.


## start playing

* prerequisites: `git`, `go`

```sh
$ git clone https://github.com/vito/bass
$ cd bass
$ go install ./cmd/bass
```

### Linux

Bass runs thunks with [Buildkit][buildkit-quickstart].

```sh
$ ./hack/start-buildkitd # if needed
$ bass ./demos/go-build-git.bass
```

[buildkit-quickstart]: https://github.com/moby/buildkit#quick-start

### macOS

macOS support works by running Buildkit in a Linux VM. This setup should be
familiar to anyone who has used Docker for Mac.

Use the included [`lima/bass.yaml`](lima/bass.yaml) template to manage the VM
with [Lima][lima].

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

### editor setup

* vim config: [bass.vim][bass.vim]

```sh
# install language server
$ go install ./cmd/bass-lsp
```

[bass.vim]: https://github.com/vito/bass.vim

```vim
Plug 'vito/bass.vim'

lua <<EOF
require'bass_lsp'.setup()
EOF
```

### cleaning up

The Buildkit runtime leaves snapshots around for caching thunks, so if you
start to run low on disk space you can run the following to clear them:

```
$ bass --prune
```


## the name

Bass is named after the :loud_sound:, not the :fish:. Please do not think of
the :fish: every time. It will eventually destroy me.


## rationale

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

It's getting there!

I'm using it for side-projects and enjoying it so far, but there's a lot of
bootstrapping still to do. The workflow feels interesting; sort of like
scripting as normal, but with a bunch of ephemeral machines instead of
polluting your local machine.

One expectation to set: this project is built for fun and is developed in my
free time. I'm just trying to build something that I would want to use. I don't
plan to bear the burden of large enterprises using it, and I'm not aiming to
please everyone.


## how can I help?

I'm really curious to hear feedback (even if things didn't go well) and ideas
for features or ways to further leverage existing ideas. Feel free to open a
new [Discussion][discussions] for these, or anything else really.

Pull requests are also welcome, but I haven't written a `CONTRIBUTING.md` yet,
and this is still a personal hobby so I will likely reject contributions I
don't want to maintain.

[discussions]: https://github.com/vito/bass/discussions


## thanks

* The [Buildkit project][buildkit], which powers the default runtime and really
  drives all the magic behind running and composing thunks.
* John Shutt, creator of the Kernel programming language.
* Rich Hickey, creator of the Clojure programming language.


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
