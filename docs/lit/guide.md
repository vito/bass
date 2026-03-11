\use-plugin{bass-www}

# guide

This guide glosses over language semantics in favor of being a quick reference
for common tasks. If you'd like to learn the language, see \reference{bassics}.

\table-of-contents

## getting started

Bass is shipped as a single [`bass`
binary](https://github.com/vito/bass/releases/latest) which needs to be
installed somewhere in your `$PATH`.

To run Bass you'll need either [Docker
Engine](https://docs.docker.com/engine/install/#server) (**Linux**),
[Docker Desktop](https://www.docker.com/products/docker-desktop/)
(**OS X**, **Windows**), or
[Buildkit](https://github.com/moby/buildkit) running.

With everything installed, try one of the demos:

\commands{{{
  bass demos/git-lib.bass
}}}

If you see `bass: command not found`, it's not in your `$PATH`.

If you see some other kind of error you're welcome to ask for help in
[GitHub](https://github.com/vito/bass/discussions/categories/q-a) or
[Discord](https://discord.gg/HFW85RyUtK).

### running thunks

\bass-literate{
  Bass is built around \t{thunks}. Thunks are cacheable commands that
  produce files and/or a stream of values.
}{{{
  (from (linux/alpine)
    ($ echo "Hello, world!"))
}}}{
  Throughout this documentation, thunks will be rendered as space invaders to
  make them easier to identify.
}{
  To run a \t{thunk}'s command and raise an error if it fails, call
  \b{run}:
}{{{
  (run (from (linux/alpine)
         ($ echo "Hello, world!")))
}}}{
  To run a thunk and get \bass{true} or \bass{false} instead of erroring,
  call \b{succeeds?}:
}{{{
  (succeeds? (from (linux/alpine)
               ($ sh -c "exit 1")))
}}}{
  Thunks are cached forever. They can be cleared with `bass --prune`,
  but this should only be necessary for regaining disk space.
}{
  If you want to run a thunk multiple times, just set a different value as
  an environment variable. Tip: use \b{now} to control cache granularity.
}{{{
  (run (with-env
         (from (linux/alpine)
           ($ echo "Hi again!"))
         {:MINUTE (now 60)}))
}}}

### reading output

\bass-literate{
  To parse a stream of JSON values from a thunk's `stdout`, call
  \b{read} with the \bass{:json} protocol:
}{{{
  (def cat-thunk
    (from (linux/alpine)
     ; note: stdin is also JSON
      (.cat "hello" "goodbye")))

  (let [stream (read cat-thunk :json)]
    [(next stream :end)
     (next stream :end)
     (next stream :end)])
}}}{
  To read output line-by-line, set the protocol to \bass{:lines}:
}{{{
  (-> ($ ls -r /usr/bin)
      (with-image (linux/alpine))
      (read :lines)
      next)
}}}{
  To parse UNIX style tabular output, set the protocol to \bass{:unix-table}:
}{{{
  (-> ($ ls -r /usr/bin)
      (with-image (linux/alpine))
      (read :unix-table)
      next)
}}}{
  To collect all output into one big string, set the protocol to \bass{:raw}:
}{{{
  (-> ($ echo "Hello, world!")
      (with-image (linux/alpine))
      (read :raw)
      next)
}}}

### providing secrets

\bass-literate{
  To shroud a string in secrecy, pass it to \b{mask} and give it a name.
}{{{
  (mask "hunter2" :nickserv)
}}}{
  Secrets can be passed to thunks as regular strings. When serialized, a
  secret's value is omitted.
}{{{
  ($ echo (mask "secret" :password))
}}}{
  \construction{Bass does not mask the secret from the command's output.
  This may come in the future.}
}{
  Sensitive values can end up in all sorts of sneaky places. Bass does its
  best to prevent that from happening.

  - A thunk's command runs in an isolated environment, so an evil thunk
    can't* spy on your secrets.
  - A thunk's command (i.e. stdin, env, argv) isn't captured into image
    layer metadata, so exporting a thunk as an OCI image will not leak
    secrets passed to it.
  - Secret values are never serialized, so publishing a thunk path will not
    leak any secrets used to build it.
  - All env vars passed to `bass` are only provided to the entrypoint
    script (as \b{script-\*env\*}). They are also *removed from the
    `bass` process* so that they can't be sneakily accessed at
    runtime.

  With the above precautions, passing secrets to thunks as env vars may
  often be most ergonomic approach. If you have more ideas, please suggest
  them!
}{
  To pass a secret to a command using a secret mount, use \b{with-mount}:
}{{{
  (-> ($ cat /secret)
      (with-mount (mask "hello" :shh) /secret)
      (with-image (linux/alpine))
      run)
}}}

\* This is all obviously to the best of my ability - I can't promise it's
perfect. If you find other ways to make Bass safer, please share them!

### caching directories

\bass-literate{
  Cache paths may be created using \b{cache-dir} and passed to thunks like
  any other path. Any data written to a cache path persists until cleared
  by `bass --prune`.
}{{{
  (def my-cache (cache-dir "my cache"))

  (defn counter [tag]
    (from (linux/alpine)
      (-> ($ sh -c "echo $0 >> /var/cache/file; cat /var/cache/file | wc -l"
             $tag)
          (with-mount my-cache /var/cache/))))

  (defn count [tag]
    (next (read (counter tag) :json)))

  [(count "once")
   (count "twice")
   (count "thrice")]
}}}{
  Currently only one thunk can access a cache path at a time. This may
  become configurable in the future.
}

## building stuff

### passing bits around

\bass-literate{
  Thunks run in an initial working directory controlled by Bass. Files
  created within this directory can be passed to other thunks by using
  \t{thunk paths}.
}{
  Thunk paths are created by using a thunk with path notation:
}{{{
  (def meowed
    (from (linux/alpine)
      (-> ($ sh -c "cat > ./file")
          (with-stdin ["hello" "goodbye"]))))

  meowed/file
}}}{
  If the thunk isn't bound to a symbol first, you can use \b{subpath}:
}{{{
  (-> ($ sh -c "cat > ./file")
      (with-image (linux/alpine))
      (with-stdin ["hello" "goodbye"])
      (subpath ./file))
}}}{
  Just like thunks, a thunk path is just an object. Its underlying thunk
  won't run until the path is needed by something.
}{
  When you \b{read} a thunk path, Bass runs its thunk and reads the content
  of the path using the same protocols for \reference{reading-output}:
}{{{
  (next (read meowed/file :json))
}}}{
  When you pass a thunk path to an outer thunk, Bass runs the path's thunk
  and mounts the path into the outer thunk's working directory under a
  hashed directory name:
}{{{
  (-> ($ ls -al meowed/file)
      (with-image (linux/alpine))
      run)
}}}{
  If the outer thunk sets a thunk path as its working directory (viw \b{cd}
  or \b{with-dir}), you can use \bass{../} to refer back to the original
  working directory.
}{{{
  (defn go-build [src pkg]
    (-> (from (linux/golang)
          (cd src
            ($ go build -o ./out/ $pkg)))
        (subpath ./out/)))

  (def cloned
    (from (linux/alpine/git)
      ($ git clone "https://github.com/vito/bass" ./repo/)))

  (go-build cloned/repo/ "./cmd/...")
}}}{
  Note that any modifications made to an input thunk path will not
  propagate to subsequent thunks.
}{
  Astute observers will note that \bass{cloned} above is not a \t{hermetic},
  because it doesn't specify a version.
}{
  The \reference{git-module} provides basic tools for cloning
  [Git](https://git-scm.com) repositories in a hermetic manner.
}{{{
  (use (.git (linux/alpine/git)))

  (let [uri "https://github.com/vito/bass"]
    (git:checkout uri (git:ls-remote uri "HEAD")))
}}}{
  The \reference{git-module} also provides \b{git-github}, a \t{path root} for
  repositories hosted at [GitHub](https://github.com).
}{{{
  git:github/vito/bass/ref/HEAD/
}}}

### troubleshooting

When something goes wrong, Bass tries to provide an ergonomic error
message. Backtraces show annotated source code complete with syntax
highlighting. When a thunk fails its output is included in the error
message at the bottom of the screen so you don't have to skim the whole
output.

\demo{multi-fail.bass}

That being said, there's a good chance you'll run into a cryptic error
message now and then while I work towards making them friendly. If you find
one, please [open an
issue](https://github.com/vito/bass/issues/new?assignees=&labels=cryptic&template=cryptic-error-message.md&title=).

### exporting files

\bass-literate{
  Thunk paths can be saved in JSON format for archival, auditing, efficient
  distribution, or just for funsies.
}{{{
  (use (.git (linux/alpine/git)))

  (-> ($ go build -o ../out/ "./cmd/...")
      (with-dir git:github/vito/bass/ref/HEAD/)
      (with-image (linux/golang))
      (subpath ./out/)
      (emit *stdout*))
}}}{
  Feeding \t{thunk path} JSON to `bass --export` will print a `tar`
  stream containing the file tree.
}

### exporting images

\bass-literate{
  Feeding \t{thunk} JSON to `bass --export` will print an OCI image
  `tar` stream, which can be piped to `docker load` for
  troubleshooting with `docker run`. \construction{This will be made
  easier in the future.}
}{{{
  (emit
    (from (linux/ubuntu)
      ($ apt-get update)
      ($ apt-get -y install git))
    *stdout*)
}}}

## special tactics

### pinning in `bass.lock` {#bass.lock}

\bass-literate{
  Bass comes with baby's first dependency pinning solution: \b{memo}.
  It works by storing results of functions loaded from Bass \t{modules}
  into a file typically called `bass.lock` and committed to your
  repository.
}{
  \b{memo} takes a `bass.lock` path, a \t{module} \t{thunk}, and a
  \t{symbol}, and returns a memoized function.
}{{{
  (def memo-ls-remote
    (memo *dir*/bass.lock (.git (linux/alpine/git)) :ls-remote))
}}}{
  Calling the function passes through to the specified function from the
  \b{load}ed module.
}{{{
  (memo-ls-remote "https://github.com/moby/buildkit" "HEAD")
}}}{
  When the function is called again with the same arguments, the cached
  response value is returned instead of making the call again:
}{{{
  (memo-ls-remote "https://github.com/moby/buildkit" "HEAD")
}}}{
  Use `bass --bump` to refresh every dependency in a `bass.lock`
  file:

  \commands{{{
    bass --bump bass.lock
  }}}

  The `bass --bump` command re-\b{load}s all embedded module thunks
  and calls each function with each of its its associated arguments,
  updating the file in-place.
}{
  Memoization is mostly leveraged for caching dependency version
  resolution. For this, your module must define the `bass.lock` path
  as a special binding: `*memos*`.
}{{{
  (def *memos* *dir*/bass.lock)
}}}{
  The \b{linux} and \b{git-github} path roots use this binding to
  automatically discover the memos location.
}{{{
  (use (.git (linux/alpine/git)))
  git:github/vito/bass/ref/main/
}}}{
  Third-party modules may respect this binding too. Here's how \b{linux} is
  defined, for reference:
}{{{
  (defop linux args scope
    (let [path-root (path {:os "linux"} (:*memos* scope null))]
      (eval [path-root & args] scope)))
}}}

### sharing bass code

Using \reference{`bass.lock`}{bass.lock} files lets you share and
reuse Bass code in `git` repos:

\demo{git-lib.bass}

### server mode {#server-mode}

\construction{I'm not sure if this is the right design for this yet, but it
seems nifty and it works. Expect this to change at any moment. Suggestions
welcome!}

To serve Bass scripts in `./srv/` over HTTP on port 6455 ("bass"), run:

\commands{{{
  bass --serve 6455 ./srv/
}}}

This is particularly handy for cobbling together endpoints for receiving
webhooks (e.g. a GitHub App for \reference{cicd}{CI/CD}).

HTTP requests sent to `http://localhost:6455/foo` will run the
`./srv/foo` Bass script.

The HTTP request sent on \b{script-\*stdin\*} as a structure like the
following:

\bass{{{
  {:headers {:Accept "application/json"}
   :body "{\"foo\":1}"}
}}}

Values emitted to \b{script-\*stdout\*} will be sent as the response. If the
script fails a `500` status code will be returned.

The UX here is very spartan at the moment. Notably there is no way to view
progress over HTTP; it's only rendered server-side in the console.

I'd like the server-side to self-update somehow, but haven't figured that
out yet.

### webhooks based CI/CD {#cicd}

The Bass project uses [Bass Loop](https://github.com/vito/bass-loop)
to receive GitHub webhooks and run its own builds. Docs coming soon - see
the [announcement](https://github.com/vito/bass-loop/discussions/1)
for now.

### pipeline based CI/CD {#pipelines}

Trigging builds on push is just one form of CI/CD. What if you have
external dependencies you'd like to trigger builds from? What if you want
to write sophisticated pipelines with fan-in and fan-out semantics?

\construction{Dunno yet! I think we're a few steps away from this, but we
need to figure out the best steps.}

Ideas for the future:

- The existing streams/pipes concepts could probably be leveraged for
  representing general-purpose concurrency.
- It's possible to use \t{streams} to model Concourse style pipelines with
  the same constraint algorithm for passing sets of versions between jobs.
