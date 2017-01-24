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

// Version is the version of walk(1)
const Version = "$(cat VERSION)"
EOF
}

case $1 in
  deps) deps ;;
  exec) gen ;;
esac

# vi:filetype=sh
