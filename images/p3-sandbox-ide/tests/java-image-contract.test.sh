#!/usr/bin/env bash
set -euo pipefail

image=${IMAGE:-p3-sandbox-ide:dev-java}
base_image=${BASE_IMAGE:-p3-sandbox-ide:dev}

base_extensions=$(docker run --rm --entrypoint code-server "$base_image" \
  --list-extensions --show-versions)
if printf '%s\n' "$base_extensions" | grep -F 'redhat.java@' >/dev/null; then
  printf 'base image unexpectedly contains redhat.java\n' >&2
  exit 1
fi

if docker run --rm --entrypoint test "$base_image" -x /opt/java/openjdk/bin/java; then
  printf 'base image unexpectedly contains the Java runtime\n' >&2
  exit 1
fi

version=$(docker run --rm --entrypoint code-server "$image" --version)
case "$version" in
  "4.132.0 "*) ;;
  *)
    printf 'unexpected code-server host: %s\n' "$version" >&2
    exit 1
    ;;
esac

extensions=$(docker run --rm --entrypoint code-server "$image" \
  --list-extensions --show-versions)
printf '%s\n' "$extensions" | grep -Fx 'redhat.java@1.55.0' >/dev/null

java_properties=$(docker run --rm --entrypoint java "$image" \
  -XshowSettings:properties -version 2>&1)
printf '%s\n' "$java_properties" | grep -F 'java.specification.version = 21' >/dev/null

docker run --rm --entrypoint cat "$image" \
  /home/coder/.local/share/code-server/Machine/settings.json |
  jq -e '
    .["files.watcherInclude"] == ["**/*", "../.ide-refresh-marker"] and
    .["files.watcherExclude"] == {} and
    .["terminal.integrated.profiles.linux"]["Sandbox VM"].path ==
      "/usr/local/bin/p3-ide-remote-shell" and
    .["terminal.integrated.defaultProfile.linux"] == "Sandbox VM"
  ' >/dev/null

docker run --rm --entrypoint cat "$image" \
  /home/coder/.local/share/code-server/User/settings.json |
  jq -e '
    .["extensions.autoCheckUpdates"] == false and
    .["extensions.autoUpdate"] == false and
    .["telemetry.telemetryLevel"] == "off" and
    .["redhat.telemetry.enabled"] == false and
    .["java.jdt.ls.java.home"] == "/opt/java/openjdk" and
    .["java.configuration.runtimes"] == [{
      "name": "JavaSE-21",
      "path": "/opt/java/openjdk",
      "default": true
    }] and
    .["java.server.launchMode"] == "Standard" and
    .["java.project.importOnFirstTimeStartup"] == "automatic" and
    .["java.configuration.updateBuildConfiguration"] == "automatic" and
    .["java.import.maven.enabled"] == true and
    .["java.import.maven.offline.enabled"] == true and
    .["java.import.gradle.enabled"] == false and
    .["java.maven.downloadSources"] == false and
    .["java.eclipse.downloadSources"] == false and
    .["java.maven.updateSnapshots"] == false
  ' >/dev/null

maven_settings=$(docker run --rm --entrypoint cat "$image" /home/coder/.m2/settings.xml)
printf '%s\n' "$maven_settings" |
  grep -F '<localRepository>/home/coder/shared/workspace/.m2/repository</localRepository>' >/dev/null
printf '%s\n' "$maven_settings" | grep -F '<offline>true</offline>' >/dev/null
