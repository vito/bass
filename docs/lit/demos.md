\use-plugin{bass-www}

# demos

These demos are available on
[GitHub](https://github.com/vito/bass/tree/main/demos):

\commands{{
git clone https://github.com/vito/bass
cd bass/
ls ./demos/
}}

## images as code

\demo{build-image.bass}

Pipe the JSON above to `bass -e | docker load` to run the image with
`docker`.

## booklit

[Booklit](https://github.com/vito/booklit) dogfoods Bass:

\demo{booklit/test.bass}
\demo{booklit/build.bass}
\demo{booklit/docs.bass}

## calculating fib {#fib-demo}

\demo{fib.bass}

## git clone => go build

\demo{go-build-git.bass}

## fetching & loading modules

\demo{git-lib.bass}

## reading a json stream

\demo{godoc.bass}

## backtraces {#errors-demo}

This is a kind of basic feature, but it was a pain in the butt to
implement, so BEHOLD!

\demo{backtrace.bass}

(It was complicated because Bass is implemented in continuation-passing
style.)
