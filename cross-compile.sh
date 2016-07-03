#!/bin/bash

TMP_DIR="$PWD/tmp"

if [ -z "$TMP_DIR" ]; then
  echo "no tmp dir"
  exit 1
fi

build() {
  for goos in darwin linux windows; do
    for goarch in 386 amd64; do
      echo "building: ${goos}-${goarch}"
      env GOOS="$goos" GOARCH="$goarch" ./build.sh > /dev/null
    done
  done
}

init() {
  rm -rf $TMP_DIR/ \
   && mkdir -p $TMP_DIR/
}

panic() {
  local message=$1
  echo $message
  exit 1
}

main() {
  init    || panic "init failed"
  build   || panic "build failed"
}
main
