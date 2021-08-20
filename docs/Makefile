targets=html/base16-options.tmpl css/main.css

all: $(targets)

html/base16-options.tmpl: css/base16/*.css scripts/base16-tmpl
	./scripts/base16-tmpl

css/main.css: less/main.less less/*.less
	yarn run lessc $< $@

clean:
	rm -f $(targets)
