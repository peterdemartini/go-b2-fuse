#!/bin/bash

APP_NAME=go-b2-fuse
TMP_DIR=$PWD/tmp
IMAGE_NAME=local/$APP_NAME

if [ -z "$TMP_DIR" ]; then
  echo "no tmp dir"
  exit 1
fi

build_on_docker() {
  docker build --tag $IMAGE_NAME:built .
}

build_on_local() {
  local goos="$1"
  local goarch="$2"
  local filename="$(get_filename "$goos" "$goarch")"
  env GOOS="$goos" GOARCH="$goarch" go build -a -tags netgo -installsuffix cgo -ldflags '-w' -o "${TMP_DIR}/${filename}" .
}

get_filename() {
  local goos="$1"
  local goarch="$2"
  local filename="${APP_NAME}"
  if [ ! -z "$goos" ]; then
    filename="${filename}-${goos}"
  fi
  if [ ! -z "$goarch" ]; then
    filename="${filename}-${goarch}"
  fi
  echo "$filename"
}

copy() {
  cp $TMP_DIR/$APP_NAME .
  cp $TMP_DIR/$APP_NAME entrypoint/
}

init() {
  rm -rf $TMP_DIR/ \
   && mkdir -p $TMP_DIR/
}

init_local() {
  local goos="$1"
  local goarch="$2"
  local filename="$(get_filename "$goos" "$goarch")"
  rm -rf $TMP_DIR/$filename \
   && mkdir -p $TMP_DIR/
}

package() {
  docker build --tag $IMAGE_NAME:latest entrypoint
}

run() {
  docker run --rm \
    --volume $TMP_DIR:/export/ \
    $IMAGE_NAME:built \
      cp $APP_NAME /export
}

panic() {
  local message=$1
  echo $message
  exit 1
}

docker_build() {
  init    || panic "init failed"
  build_on_docker || panic "build_on_docker failed"
  run     || panic "run failed"
  copy    || panic "copy failed"
  package || panic "package failed"
}

local_build() {
  local goos="$1"
  local goarch="$2"

  init_local "$goos" "$goarch" || panic "init failed"
  build_on_local "$goos" "$goarch" || panic "build_on_local failed"
}

main() {
  local mode="$1"

  local goos="${GOOS}"
  local goarch="${GOARCH}"

  if [ "$mode" == "-h" -o "$mode" == "--help" -o "$mode" == "help" ]; then
    echo "Usage: ./build.sh (local|docker)"
    echo "Default build is 'local'"
    exit 1
  fi

  if [ "$mode" == "docker" ]; then
    docker_build
    exit $?
  fi

  local_build "$goos" "$goarch"
}

main $@
