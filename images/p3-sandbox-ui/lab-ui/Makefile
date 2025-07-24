all: build

build: dist/.keep

dist/.keep: src/index.html src/app.js src/favicon.webp scripts/build.sh ../bootstrap-theme/dist/.keep ../github-markdown-css/dist/.keep
	./scripts/build.sh
	touch dist/.keep

../bootstrap-theme/dist/.keep:
	$(MAKE) -C ../bootstrap-theme build

../github-markdown-css/dist/.keep:
	$(MAKE) -C ../github-markdown-css build

.PHONY: serve
serve: build
	npx -y live-server dist

.PHONY: clean
clean:
	rm -rf dist build
