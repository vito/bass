\use-plugin{bass-www}

# stdlib {#stdlib}

The standard library is the set of \t{modules} that come with Bass.

\table-of-contents

## ground module {#ground}

The ground module is inherited by all modules. It provides basic language
constructs and the standard toolkit for running \t{thunks}.

\ground-docs

### script bindings

Bass modules always run as commands. Either a user runs the script with the
`bass` command, or another Bass module runs it as a \t{thunk}.

When a module is run as a script, the values reflect the system values
available to the `bass` command, and \b{script-main} is called with
the arguments passed to `bass`.

When a module is run as a thunk, the values reflect the values set in the
thunk, and \b{script-main} is called with the thunk's args.

\script-docs

## `.strings` module {#strings-module}

Simple functions for manipulating UTF-8 encoded strings.

\stdlib-docs{strings}{{{(load (.strings))}}}

## `.git` module {#git-module}

Bare essentials for fetching [Git](https://git-scm.com) repositories,
using the `git` CLI from an image passed on `stdin`.

This module is limited to functions necessary for fetching other Bass
scripts, i.e. bootstrapping.

\stdlib-docs{git}{{{(load (.git (linux/alpine/git)))}}}

## `.time` module {#time-module}

\stdlib-docs{time}{{{(load (.time))}}}

## `.regexp` module {#regexp-module}

\stdlib-docs{regexp}{{{(load (.regexp))}}}
