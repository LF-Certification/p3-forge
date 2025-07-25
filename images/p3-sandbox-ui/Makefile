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
	cd dev && docker compose up -d

container: build
	docker build -t ghcr.io/lf-certification/lab-ui .

push-container: container
	docker push ghcr.io/lf-certification/lab-ui
