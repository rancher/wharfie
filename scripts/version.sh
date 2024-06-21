#!/bin/bash

if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    DIRTY="-dirty"
fi

COMMIT=$(git rev-parse --short HEAD)
GIT_TAG=${GITHUB_ACTION_TAG:-$(git tag -l --contains HEAD | head -n 1)}

if [[ -z "$DIRTY" && -n "$GIT_TAG" ]]; then
    VERSION=$GIT_TAG
else
    VERSION="${COMMIT}${DIRTY}"
fi

if [ -z "$ARCH" ]; then
    ARCH=$(go env GOARCH)
fi

if [ ${ARCH} = armv7l ] || [ ${ARCH} = arm ]; then
    export GOARCH="arm"
    export GOARM="7"
fi
