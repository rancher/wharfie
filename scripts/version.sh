#!/bin/bash

if [ -n "$DRONE_TAG" ]; then
  VERSION="$DRONE_TAG"
else 
  git fetch --tags &>/dev/null || true
  GIT_TAG=$(git describe --tags --dirty 2>/dev/null)
  if [ -n "$GIT_TAG" ]; then
    VERSION="$GIT_TAG"
  else
    VERSION="v0.0.0-$(git describe --always --dirty)"
  fi
fi

if [ -z "$ARCH" ]; then
    ARCH=$(go env GOARCH)
fi

if [ ${ARCH} = armv7l ] || [ ${ARCH} = arm ]; then
    export GOARCH="arm"
    export GOARM="7"
fi
