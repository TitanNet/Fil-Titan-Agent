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

build_and_package() {
  local component_dir=$1
  local output_name=$2

  cd "$component_dir" || exit 1

  # 构造 go build 命令
  if [[ -n "$TAG" ]]; then
    echo "Building with tag: $TAG"
    GOOS=darwin GOARCH=arm64 go build -trimpath -tags "$TAG" || { echo "Build failed for $component_dir"; exit 1; }
    output_name="${output_name}-${TAG}"  # 在 zip 文件名中加上 tag
  else
    GOOS=darwin GOARCH=arm64 go build -trimpath || { echo "Build failed for $component_dir"; exit 1; }
  fi

  # Package the binary into a zip file
  zip "${output_name}.zip" "./$(basename "$component_dir")" || { echo "Zipping failed for $component_dir"; exit 1; }

  # Move the zip to the release directory
  mv "${output_name}.zip" "../../$RELEASE_DIR/" || { echo "Move failed for $component_dir"; exit 1; }

  # Remove the binary to clean up
  rm "./$(basename "$component_dir")" || { echo "Cleanup failed for $component_dir"; exit 1; }

  # Return to the root directory
  cd - > /dev/null || exit 1
}

# Build and package agent
build_and_package "$AGENT_DIR" "agent-darwin-arm64"

# Build and package controller
build_and_package "$CONTROLLER_DIR" "controller-darwin-arm64"

# 生成并显示 MD5 校验码
md5 "$RELEASE_DIR/agent-darwin-arm64.zip"
md5 "$RELEASE_DIR/controller-darwin-arm64.zip"
