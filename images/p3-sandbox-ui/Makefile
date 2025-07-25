.PHONY: all build clean serve clean-serve down

all: build

build:
	$(MAKE) -C github-markdown-css build
	$(MAKE) -C bootstrap-theme build
	$(MAKE) -C lab-ui build

clean:
	$(MAKE) -C github-markdown-css clean
	$(MAKE) -C bootstrap-theme clean
	$(MAKE) -C lab-ui clean
	-cd dev && docker compose down

down:
	-cd dev && docker compose down --volumes --remove-orphans

serve: build
	cd dev && docker compose up

test-local: build
	./test-local.sh

test-ui-config: build
	@echo "Testing UI_CONFIG environment variable..."
	docker build -t p3-sandbox-ui-test .
	docker run --rm -p 8080:80 \
		-e 'UI_CONFIG={"tools": [{"id": "terminal", "title": "Test Terminal", "url": "/terminal/", "default": true}]}' \
		p3-sandbox-ui-test

container: build
	docker build -t ghcr.io/lf-certification/lab-ui .

push-container: container
	docker push ghcr.io/lf-certification/lab-ui
