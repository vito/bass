# syntax = basslang/frontend:dev

(use (*dir*/bass/bass.bass))

(def dist
  (bass:dist *context* "dev" "linux" "amd64"))

(-> (from (linux/alpine)
      ($ cp dist/bass /usr/local/bin/bass))
    (with-entrypoint ["bass"]))
