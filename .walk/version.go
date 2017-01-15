#!/bin/bash

target=$2

deps() {
  echo VERSION
}

gen() {
  cat <<EOF > "$target"
// DO NOT MODIFY
// Auto generated from by 'walk version.go'
package main

const Version = "$(cat VERSION)"
EOF
}

case $1 in
  deps) deps ;;
  exec) gen ;;
esac

# vi:filetype=sh
