all: build

build: dist/.keep

dist/.keep: github-markdown.css
	mkdir -p dist
	cp github-markdown.css dist/
	npx -y uglifycss github-markdown.css > dist/github-markdown.min.css
	touch dist/.keep

.PHONY: clean
clean:
	rm -rf dist
