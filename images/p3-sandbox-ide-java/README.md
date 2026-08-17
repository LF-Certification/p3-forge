# P3 Sandbox Java IDE

Java 21 variant of [`p3-sandbox-ide`](../p3-sandbox-ide/README.md) for sandbox workspaces that need Java language intelligence.

## Included tooling

- Eclipse Temurin JDK 21
- Red Hat Java extension for code-server
- Maven project import and dependency-source indexing

The image inherits the base IDE entrypoint, SSHFS workspace behavior, integrated terminal, and environment-variable contract. Sandbox blueprints select it through `ide.version: java-v1`; the sandbox operator maps that version to this image.

## Offline contract

Candidate sessions are not expected to download JDKs, extensions, Maven artifacts, plugins, or sources. The content build must prewarm the target VM user's Maven repository. The sandbox operator copies that prepared repository into writable pod-local storage mounted at `/home/coder/.m2/repository` before the IDE starts.

Maven import runs offline in the IDE. Authoritative Maven builds and tests run through the integrated terminal on the target VM.

## Local verification

```bash
docker build --tag p3-sandbox-ide-java:dev images/p3-sandbox-ide-java
images/p3-sandbox-ide-java/tests/image-contract.test.sh
```
