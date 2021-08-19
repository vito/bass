# bass

Bass is a Lisp dialect influenced by the [Kernel programming language][kernel],
with naming conventions shamelessly stolen from [Clojure][clojure].

Bass is __not__ a general-purpose programming language. Bass is a low-fidelity
scripting language for automating the infrastructure beneath your project. Bass
harmonizes with other languages to do all the high-fidelity work, by running
them as cacheable workloads in a container runtime.

Bass's goal is to make automating the path to production safe, easy (ish),
predictable, verifiable, and fun.

## install & run

* prerequisites: `git`, `go`, `docker`

```sh
$ git clone https://github.com/vito/bass
$ cd bass
$ go install ./cmd/bass
$ bass
=> (log "hello")
```

## examples

This example uses [Concourse resources][resources] and [tasks][tasks] to fetch,
test, and build a git repository using the `.concourse` module from the Bass
standard library.

```clojure
; import Concourse library
(import (load (.concourse))
        resource
        get-latest
        run-task)

; define a Concourse resource
(def booklit
  (resource linux :git {:uri "https://github.com/vito/booklit"}))

; fetch latest repo
(def latest-booklit
  (get-latest booklit))

; run tests
(run-task latest-booklit/ci/test.yml
          :inputs {:booklit latest-booklit})

; build assets
(let [result (run-task latest-booklit/ci/build.yml
                       :inputs {:booklit latest-booklit})]
  (emit result:outputs:assets *stdout*))
```

The `(emit ... *stdout*)` line at the bottom emits a workload path in JSON
format to `stdout`. The JSON payload is a recipe for building the same
artifact, including all of its inputs, recursively. It could be written to a
file, or extracted to a local directory with `bass --export`:

```sh
$ ./example | jq .
{"workload":{...},"path":{"dir":"assets"}}
$ ./example > assets.json
$ ./example | bass --export | tar -xf -
```

More demos are included under [`demos/`](demos/).

* [`demos/booklit/docs.bass`](demos/booklit/docs.bass) fetches a `git`
  Concourse resource and runs a script from the repo as a workload:

  ```sh
  $ ./demos/booklit/docs.bass | bass -e | tar -tf -
  # ...
  ./
  ./plugins.html
  ./html-renderer.html
  ./getting-started.html
  ./baselit.html
  ./index.html
  ./booklit-syntax.html
  ./thanks.html
  ```

  Try running the command again - it should be fast after the first run!

* [`demos/booklit/test.bass`](demos/booklit/test.bass) fetches the same repo
  and runs its [`ci/test.yml`][booklit-test] Concourse task.

  ```sh
  $ ./demos/booklit/test.bass | bass -e | tar -tf -
  # ...
  Test Suite Passed
  ```

* [`demos/booklit/build.bass`](demos/booklit/build.bass) fetches the same repo
  and runs its [`ci/build.yml`][booklit-build] Concourse task.

  ```sh
  $ ./demos/booklit/build.bass | bass -e | tar -tf -
  # ...
  ./
  ./booklit_linux_amd64
  ./booklit_darwin_amd64
  ./booklit_windows_amd64.exe
  ```

## the name

Bass is named after the :loud_sound:, not the :fish:. Please do not think of
the :fish: every time. It will eventually destroy me.


## rationale

A Lisp dialect for running other languages in containers may seem like a
solution in search of a problem, so I'll try to explain how I got here.

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

Nope. I'll update this if the situation changes.

This project is built for fun. If it makes its way into real-world use cases,
that's cool, but right now I'm just trying to build something that I would want
to use. I don't plan to bear the burden of large enterprises using it, and I'm
not going to try to please everyone.


## cool stuff

> This section is a mess at the moment - I'll tidy it up once I have somewhere
> else to put these thoughts.

### path types

Bass has built-in types for representing paths to files, directories, or
commands (i.e. resolved by consulting `$PATH`).

```clojure
=> ./foo
./foo
=> ./dir/
./dir/
=> (def x ./dir/)
x
=> x/file
./dir/file
```

Paths can either be context-free literals like `./file` or `./dir/`, or
abstract path values representing reproducible paths created by running a
workload.

### polyglot runtime model via workloads

Bass programs construct, run, and cache workloads with a calling convention
built on universal standards like [OCI images][oci], [JSON][json], and [Unix
streams][streams].

Typical workloads might run tests, build artifacts, or do some computation and
return a sequence of values. Workloads are content-addressed and cacheable; the
same workload will always have the same identifier and must always yield the
same result (for all intents and purposes).

A workload is constructed by calling a path value representing the command or
file to run. Any arguments provided will be passed to the command as a JSON
stream on `stdin`.

```clojure
=> (.hello "world!" 42 true)


      ██  ██  ██
    ██████████████
  ████    ██    ████
  ██████████████████
    ██████  ██████
  ████    ██    ████
    ████      ████


{:path .hello :response {:stdout true} :stdin ("world!" 42 true)}
```

Internally, a workload is just an object fulfilling a schema interpreted by the
runtime. In the REPL, Bass will helpfully print a colorful space invader to
help visually distinguish them from one another. (I haven't determined whether
it's actually helpful, but it's cool, so it stays.)

The above workload doesn't have an image, so to run it Bass will resolve
`hello` to `hello.bass` somewhere in the Bass load path and evaluate it
in-process. Which probably won't work.

To run a workload in a container, the workload must specify an image and a
platform. Helper functions like `in-image`, `on-platform`, and `with-args` can
be used to set the appropriate fields in the workload.

The primitive for running workloads is `(run)`. It returns a stream source from
which values can be read with `(next)`.

```clojure
=> (def jq-a
     (-> (.jq {:a 1} {:a 2} {:a 3})
         (with-args ".a")
         (in-image "vito/jq")))
jq-a
=> (run jq-a)
11:27:32.218    info    running {"workload": "58d6191b29932be3cf22b2366e10a4a860f2b352"}
11:27:32.256    info    created {"workload": "58d6191b29932be3cf22b2366e10a4a860f2b352", "container": "273c292fb908f4425c2c3ab2f8c66ab37a623da84d9cdd514e65ed54f34c9f5a"}
11:27:32.886    debug   removed {"workload": "58d6191b29932be3cf22b2366e10a4a860f2b352", "container": "273c292fb908f4425c2c3ab2f8c66ab37a623da84d9cdd514e65ed54f34c9f5a"}
=> (each log (run jq-a))
15:34:10.196    debug   cached  {"workload": "58d6191b29932be3cf22b2366e10a4a860f2b352", "response": "/home/vito/.cache/bass/responses/58d6191b29932be3cf22b2366e10a4a860f2b352"}
11:27:32.887    info    1
11:27:32.887    info    2
11:27:32.887    info    3
null
=>
```

Notice that we called `(run)` twice. The second time it was cached, and
`(each)` just looped over its cached response.

### reproducible artifacts via workload paths

A workload path is a path paired with the workload that creates it.

Whereas Concourse builds ostensibly reproducible artifacts and persists them in
the cluster, Bass artifacts are built and passed around _by value_, where the
value contains the workload structure that created it, including all of its
input artifacts, recursively.

Bass workloads run in a fresh working directory in which the process may place
data to be cached. Workload paths refer to paths within the working directory.

```clojure
=> (-> ($ .git clone "https://github.com/vito/booklit" ./repo/)
       (in-image "bitnami/git")
       (path ./repo/))
```

Workload paths can be constructed before even running the workload; they are
lazily evaluated by the runtime and mounted to the container when it's needed.

Workload paths represent reproducible artifacts. They can be passed from
workload to workload, extracted to the host machine, or even dumped in JSON
form.

### verifiable artifacts

Artifacts built by Bass can be delivered alongside a single `.json` file that
consumers can run to reproduce the same artifact, including all of its inputs.
This `.json` file can also be used to verify everything that went into
the final artifact, which is helpful for auditing CVE exposure.

### polyglot programming via workloads

The primitive for building workloads is to call a (non-directory) path value.

The calling convention is designed to be completely decoupled from the Bass
language semantics, and simple enough to be consumed and implemented by any
other language. I would be happy to see this convention adapted to other host
languages that compete with or replace Bass.

### importing workload paths

Instead of implementing its own package ecosystem, Bass allows you to `(load)`
and `(run)` Bass workloads fetched by another workload.

```clojure
(import (load (.concourse)) resource get-latest)

(def bass
  (get-latest
    (resource linux :git {:uri "https://github.com/vito/bass"})))

(import (load (bass/std/strings)) join)

(log (join " " ["hello," "world!"]))
```

This is a bit of a cop-out, but it frees me (or anyone) from having to maintain a
central package repository, and leverages capabilities Bass already has.

### command interpreters

A command interpreter looks and feels sort of like an object fulfilling an
interface, but under the hood is actually implemented as a closure that
dispatches on a command + arguments to run a workload or construct a workload
path.

For example, a Concourse resource is implemented as a command interpreter which
understands the `.check`, `.get`, and `.put` commands.

```clojure
=> (import (load (.concourse)) resource)
<env>
=> (def booklit
     (resource "concourse/git-resource"
               :uri "https://github.com/vito/booklit"))
=> (last (booklit .check))
{:ref "4b4c28077275c1bbe35e9423cf2e0e9010961f45"}
```

A commandline interface is preferable to manually constructing and running
workloads, and works well in situations where OCI images are written to conform
to preconcieved interfaces such as the Concourse resource type interface.

### significant comments

Comments are significant syntax, acting as a lightweight documentation system.
Comments adjacent to forms which evaluate to symbols (i.e. `def`) are
associated to the symbol in the environment, visible by calling `(doc)`.

```clojure
=> (def a 42) ; the answer
a
=> (doc a)
--------------------------------------------------
a number?

the answer

null
=>
```

Comments are also shown in stack traces, so they can be used to warn teammates
about potential failures ahead of time - they'll show up directly in the
failure output!

### tail recursion

Bass is implemented in continuation-passing style in order to support tail
recursion. Theoretically, this means Bass supports advanced control flow with
first-class continuations, but it isn't exposed to the language at the moment.
(It's too powerful and scary.)


## thanks

* John Shutt, creator of the Kernel programming language which
  [inspired][pumice] [many][cletus] [implementations][pumice] preceding Bass. I
  learned a lot from it!
* Rich Hickey, creator of the Clojure programming language.


[kernel]: https://web.cs.wpi.edu/~jshutt/kernel.html
[clojure]: https://clojure.org/
[concourse]: https://github.com/concourse/concourse
[oci]: https://github.com/opencontainers/image-spec
[pumice]: https://github.com/vito/pumice
[cletus]: https://github.com/vito/cletus
[hummus]: https://github.com/vito/hummus
[resources]: https://concourse-ci.org/resources.html
[tasks]: https://concourse-ci.org/tasks.html
[jq]: https://stedolan.github.io/jq/
[concourse-types]: https://resource-types.concourse-ci.org/
[streams]: https://en.wikipedia.org/wiki/Standard_streams

[booklit-test]: https://github.com/vito/booklit/blob/master/ci/test.yml
[booklit-build]: https://github.com/vito/booklit/blob/master/ci/build.yml
