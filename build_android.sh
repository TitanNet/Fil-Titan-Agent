#!/bin/bash

# 相对路径定义
RELEASE_DIR="script/bin"
AGENT_DIR="cmd/agent"
CONTROLLER_DIR="cmd/controller"

# 读取命令行参数
TAG=""

while getopts "t:" opt; do
  case ${opt} in
    t )
      TAG=$OPTARG
      ;;
    \? )
      echo "Usage: $0 [-t tag]"
      exit 1
      ;;
  esac
done

# 设置 Android NDK 和交叉编译工具链
ANDROID_NDK_HOME="/opt/android-ndk-r27-beta1"
ANDROID_CC="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/armv7a-linux-androideabi34-clang"

build_and_package() {
  local component_dir=$1
  local output_name=$2
  local module=$3

  cd "$component_dir" || exit 1

  # 设置环境变量
  export ANDROID_NDK_HOME=$ANDROID_NDK_HOME
  export CC=$ANDROID_CC

  # 构建命令 - 如果指定了 TAG，则将其作为构建标签
  if [[ -n "$TAG" ]]; then
    echo "Building with tag: $TAG"
    CGO_ENABLED=1 GOOS=android GOARCH=arm GOARM=7 go build -trimpath -tags "$TAG" || exit 1
    # CGO_ENABLED=0 GOOS=android GOARCH=arm64 GOARM=8 go build -trimpath -tags "$TAG" || exit 1
    output_name="${output_name}-${TAG}"  # 在 zip 文件名中加上 tag
  else
    CGO_ENABLED=1 GOOS=android GOARCH=arm GOARM=7 go build -trimpath || exit 1
    # CGO_ENABLED=0 GOOS=android GOARCH=arm64 GOARM=8 go build -trimpath || exit 1
  fi

  # 打包成 zip 文件
  zip "${output_name}.zip" "$module" || exit 1
  mv "${output_name}.zip" "../../$RELEASE_DIR/" || exit 1

  # 清理生成的二进制文件
  rm "$module" || exit 1

  cd - > /dev/null || exit 1
}

# 构建并打包 agent
build_and_package "$AGENT_DIR" "agent-android" "agent"

# 构建并打包 controller
build_and_package "$CONTROLLER_DIR" "controller-android" "controller"

# 生成并显示 MD5 校验码
md5sum "$RELEASE_DIR/agent-android.zip"
md5sum "$RELEASE_DIR/controller-android.zip"
md5sum "$RELEASE_DIR/agent-android-box.zip"
md5sum "$RELEASE_DIR/controller-android-box.zip"

