all: build

build: dist/.keep

dist/.keep: scss/custom.scss scss/_variables.scss
	npm install
	npm run build
	npx -y uglifycss dist/custom.css > dist/custom.min.css
	touch dist/.keep

.PHONY: clean
clean:
	rm -rf dist node_modules
