# AGENTS.md

Notes for AI coding agents (and humans) working in this repo.

This repo holds the source files for the container images used with the P3
Sandbox Operator. Each image is versioned and released on its own.

## This repo is public

Everything committed here is readable by anyone, including this file. Keep out
employer or client details, internal hostnames and URLs, cloud account or
cluster identifiers, ticket contents, customer data, and credentials.

Take extra care when you arrive from one of the private repos — carrying a
snippet across is the easy way to leak something. Move only what the image
itself needs, and rewrite it in general terms.

The images are published too, so a Dockerfile or script that bakes in a secret
or an internal endpoint hands it to anyone who pulls the image, not just to
anyone who reads the repo.

## Releasing an image

Read `VERSIONING.md` before cutting a release. Releases go through
`make release images/<name>`, which works out the next version from the
Conventional Commit messages, regenerates that image's changelog, commits it,
and creates an annotated tag. Preview any of it with
`make release-dry-run images/<name>`.

Do not write a release tag by hand or push a raw tag ref. Doing so skips the
changelog and the release commit, and a published `vX.Y.Z` tag is frozen — the
only way back is a fix-forward commit.

Only a tag push builds a release image. Merging to main builds dev images
instead, which is the cheap way to test a change before you release it.

## Commits

This repo uses [Conventional Commits](https://www.conventionalcommits.org/),
enforced by a `commit-msg` hook. The type you choose drives the next version
number, so the commit message decides the release.

Take care with squash merges: one squashed commit can span several images, and
a `BREAKING CHANGE:` footer meant for one of them will bump and label all of
them.

## Tooling

Development tools come from devbox. If a binary is not on your `PATH`, reach
for it with `devbox run -- <command>` rather than installing it.
