#!/bin/bash

set -euo pipefail

GOOS_LIST=("linux" "windows" "darwin")
GOARCH_LIST=("amd64" "arm64")
BUILD_NAME=${BUILD_NAME:-"git"}
BUILD_ANNOTATION="$(date --iso-8601=seconds)"
BUILD_NAME_VAR_PACKGE="github.com/circle-makotom/orbs-sync/cmd"

if [ "${GOOS_LIST_OVERRIDE:-}" ]
then
    eval GOOS_LIST="${GOOS_LIST_OVERRIDE}"
fi
if [ "${GOARCH_LIST_OVERRIDE:-}" ]
then
    eval GOARCH_LIST="${GOARCH_LIST_OVERRIDE}"
fi

build_gc() {
    goos=$1
    goarch=$2

    GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 go build -ldflags "-X ${BUILD_NAME_VAR_PACKGE}.BuildName=${BUILD_NAME} -X ${BUILD_NAME_VAR_PACKGE}.BuildAnnotation=${BUILD_ANNOTATION}" -o "dist/${goos}/${goarch}/orbs-sync" .

    if [ "${goos}" == "windows" ]
    then
        mv "dist/${goos}/${goarch}/orbs-sync" "dist/${goos}/${goarch}/orbs-sync.exe"
    fi
}

build_android() {
    goos=$1
    goarch=$2

    if [ ! -d android-ndk-r21e ]
    then
        # https://developer.android.com/ndk/downloads
        curl -JOL https://dl.google.com/android/repository/android-ndk-r21e-linux-x86_64.zip
        echo 'c3ebc83c96a4d7f539bd72c241b2be9dcd29bda9 android-ndk-r21e-linux-x86_64.zip' | sha1sum -c
        unzip android-ndk-r21e-linux-x86_64.zip
    fi

    if [ "${goarch}" == "arm64" ]
    then
        arch_clang="aarch64"
    elif [ "${goarch}" == "amd64" ]
    then
        arch_clang="x86_64"
    fi

    CC="$(pwd)/android-ndk-r21e/toolchains/llvm/prebuilt/linux-x86_64/bin/${arch_clang}-linux-android30-clang" \
    CXX="$(pwd)/android-ndk-r21e/toolchains/llvm/prebuilt/linux-x86_64/bin/${arch_clang}-linux-android30-clang++" \
    GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=1 go build -ldflags "-X ${BUILD_NAME_VAR_PACKGE}.BuildName=${BUILD_NAME} -X ${BUILD_NAME_VAR_PACKGE}.BuildAnnotation=${BUILD_ANNOTATION}" -o "dist/${goos}/${goarch}/orbs-sync" .
}

package() {
    goos=$1
    goarch=$2

    pushd "dist/${goos}/${goarch}"
    if [ "${goos}" == "linux" ] || [ "${goos}" == "android" ]
    then
        tar -czf "../../orbs-sync-${BUILD_NAME}-${goos}-${goarch}.tar.gz" .
    else
        zip -r "../../orbs-sync-${BUILD_NAME}-${goos}-${goarch}.zip" .
    fi
    popd
}

for goos in "${GOOS_LIST[@]}"
do
    for goarch in "${GOARCH_LIST[@]}"
    do
        echo "${goos}/${goarch}"

        mkdir -p "dist/${goos}/${goarch}"

        if [ "${goos}" == "android" ]
        then
            build_android "${goos}" "${goarch}"
        else
            build_gc "${goos}" "${goarch}"
        fi

        package "${goos}" "${goarch}"
    done
done
