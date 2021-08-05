# bass

> a low fidelity scripting language for building reproducible artifacts

## install

```sh
$ go get github.com/vito/bass/cmd/bass
```

## run

```sh
$ bass
```

## demos

* prerequisites: `git`, `go`, `docker`

```sh
$ git clone https://github.com/vito/bass
$ go install ./cmd/bass
$ bass ./demos/godoc.bass
02:30:04.626    info    running {"workload": "f8c0b959f8152f9e408bd3371294b4cd3757be62"}
02:30:04.676    info    created {"workload": "f8c0b959f8152f9e408bd3371294b4cd3757be62", "container": "4b8e35dcaccfddbd49d5830e76c663ae05f3a3bc527b001c7782697a3e568abd"}
02:30:05.558    info    Package testing provides support for automated testing of Go packages.
$ bass ./demos/resource.bass
# ...
```
