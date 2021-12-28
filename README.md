# bass

> a low-fidelity Lisp dialect for running cacheable commands and delivering
> reproducible artifacts.

> **NOTE**: there have been significant changes recently and this README is a
> bit stale. see [`9b78421`][buildkit-ref] and [#16][host-paths-pr] for now!

[buildkit-ref]: https://github.com/vito/bass/commit/9b784210af88c9f65bcac08459654a229530d9ec
[host-paths-pr]: https://github.com/vito/bass/pull/16

<p align="center">
  <img src="https://raw.githubusercontent.com/vito/bass/main/demos/readme.gif">
</p>

## start playing

* prerequisites: `git`, `go`, `docker`

```sh
$ git clone https://github.com/vito/bass
$ cd bass
$ go install ./cmd/bass
$ bass ./demos/booklit/test.bass
$ bass
=> (log "hello world!")
```

### editor setup

* vim config: [bass.vim](https://github.com/vito/bass.vim)
* language server: `go install ./cmd/bass-lsp`

```vim
Plug 'vito/bass.vim'

lua <<EOF
require'bass_lsp'.setup()
EOF
```

## reasons you might be interested

* you'd like to have a reproducible, uniform stack betwen dev and CI
* you're sick of YAML and want to write code instead of config and templates
* you'd like be able to audit and rebuild published artifacts
* you think repeatable builds are the bee's knees
* you're nostalgic about Lisp
* you're just looking for a fun project to hack on

## in a nutshell

Bass's goal is to make the path to production predictable, verifiable,
flexible, and most importantly, fun.

<!--
Bass is a Lisp dialect strongly influenced by [Kernel] and [Clojure]. It's
implemented in [Go], but that's neither here nor there. The language is tiny
(albeit underspecified) and other implementations are welcome.
-->

Bass programs are all about running thunks. A thunk is a command to run in a
container that may yield artifacts and/or a stream of response values. Thunks
must be hermetic and idempotent; they are cached aggressively, but may need to
re-run in certain situations.

```clojure
(run
  (from "alpine"
    ($ echo "Hello, world!")))
```

The runtime runs the thunk's command and keeps track of any data written to its
working directory. Paths under the thunk's working directory can be passed to
other commands just as easily as scalar values:

```clojure
(defn file [str]
  (-> (from "alpine"
        ($ sh -c "echo \"$0\" > file" $str))
      (path ./file)))

(run
  (from "alpine"
    ($ cat (file "Hello, world!"))))
```

As artifacts from one thunk are passed to the next, and outputs from that thunk
are passed to the next one, the thunk grows larger and larger - ultimately
containing every single input that went into the final artifact. At the end of
your pipeline you may choose to publish the thunk itself - sort of like serving
the recipe with a meal.

```clojure
(dump
  (from "alpine"
    ($ cat (file "Hello, world!"))))
```

Having a point-in-time snapshot of your built artifact makes it easier to
verify what exactly went into production, which could be handy when the next
CVE drops.


## example

```clojure
(run
  (from "alpine"
    ($ echo "Hello, world!")))

(defn git-deref [uri ref]
  ; note: (-> x a b c) is sugar for (c (b (a x)))
  (-> (from "alpine/git"
        ($ git ls-remote $uri $ref))
      (response-from :stdout :unix-table) ; parse awk-style output
      (with-label :at (now 60)) ; cache with minute granularity
      run
      next
      first))

(defn git-clone [uri sha]
  (-> (from "alpine/git"
        ($ git clone $uri ./)
        ($ git checkout $sha))
      (path ./)))

(defn go-build [src pkg]
  (-> (from "golang"
        (cd src
          ($ go build -o ../out/ $pkg)))
      (path ./out/)))

(def bass
  "https://github.com/vito/bass")

(-> (git-clone bass (git-deref bass "main"))
    (go-build "./...")
    (emit *stdout*))
```

The `(emit *stdout*)` call at the bottom emits the thunk path returned by
`(go-build)` in JSON format to `stdout`. This payload is a recipe for building
the same artifact, including all of its inputs, recursively. It could be written
to a `.tar` file or extracted to a local directory with `bass --export`:

```sh
$ ./example | jq .
{"thunk":{...},"path":{"dir":"out"}}
$ ./example > assets.json                  # save reproducible artifact file
$ ./example | bass --export                # display content instead
$ ./example | bass --export > example.tar  # save archive
$ ./example | bass --export | tar -xf -    # extract to cwd
```

For more demos, see [demos ; bass](https://vito.github.io/bass/demos.html).

## the name

Bass is named after the :loud_sound:, not the :fish:. Please do not think of
the :fish: every time. It will eventually destroy me.


## rationale

A Lisp dialect for running commands may seem like a solution in search of a
problem, so I'll try to explain how I got here.

### what problem does this solve?

In short, I'm taking another crack at [Concourse][concourse]'s problem space by
approaching it from a different angle. My goal with Bass is to build a language
that can express everything Concourse has been trying to, but with a fraction
of the maintenance and operational burden.

Bass is just an interpreter. There is no database, no blobstore, no API, and no
web UI (though a visualizer would be neat). Bass scripts can be run from
developer machines, from within a CI/CD system, or as a standalone continuous
build daemon. Bass is whatever you write with it.

### why a new Lisp?

I think the pattern of YAML DSLs interpreted by DevOps services may be evidence
of a gap in our toolchain that could be filled by something more expressive.
I'm trying to discover a language that fills that gap while being small enough
to coexist with all the other crap a DevOps engineer has to keep in their head.

After writing enterprise cloud software for so long, it feels good to return to
the loving embrace of `(((one thousand parentheses)))`. For me, a good Lisp is
the most fun you can have with programming. Lisps are known for doing a lot
with a little - which is exactly what I need for this project.

#### Kernel's influence

Bass is a descendant of the [Kernel programming language][kernel]. Kernel is
the tiniest Lisp dialect I know of - it has a primitive form _beneath_
`lambda`! The same mechanism replaces the conventional macro system from other
Lisps. Kernel's evaluation model makes it difficult to optimize for production
applications, but Bass targets a domain where its own performance won't be the
bottleneck, making it a rare opportunity to share Kernel's ideas with the
world.

#### Clojure's influence

Bass steals conventions from Clojure, because you should never have to tell a
coworker that the function to get the first element of a list is called :car:. A
practical Lisp dialect should do its best to bridge the gap from the past to
the future by being accessible to anyone, not just language nerds. Bass marries
Kernel's language semantics with Clojure's ease of use, including things like
the `->` macro.


## is it any good?

Nope. I'll update this if the situation changes. (**Update 2021-12-28**: It's
getting there!)

This project is built for fun. If it makes its way into real-world use cases,
that's cool, but right now I'm just trying to build something that I would want
to use. I don't plan to bear the burden of large enterprises using it, and I'm
not going to try to please everyone.


## thanks

* John Shutt, creator of the Kernel programming language which
  [inspired][pumice] [many][cletus] [implementations][hummus] preceding Bass. I
  learned a lot from it!
* Rich Hickey, creator of the Clojure programming language.


[kernel]: https://web.cs.wpi.edu/~jshutt/kernel.html
[clojure]: https://clojure.org/
[go]: https://golang.org
[concourse]: https://github.com/concourse/concourse
[oci]: https://github.com/opencontainers/image-spec
[pumice]: https://github.com/vito/pumice
[cletus]: https://github.com/vito/cletus
[hummus]: https://github.com/vito/hummus
[resources]: https://concourse-ci.org/resources.html
[tasks]: https://concourse-ci.org/tasks.html
[jq]: https://stedolan.github.io/jq/
[concourse-types]: https://resource-types.concourse-ci.org/
[json]: https://www.json.org/
[streams]: https://en.wikipedia.org/wiki/Standard_streams

[booklit-test]: https://github.com/vito/booklit/blob/master/ci/test.yml
[booklit-build]: https://github.com/vito/booklit/blob/master/ci/build.yml
