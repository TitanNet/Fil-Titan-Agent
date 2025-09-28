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
  
  # 编译二进制文件，使用 TAG 如果指定了
  if [[ -n "$TAG" ]]; then
    echo "Building with tag: $TAG"
    go build -trimpath -tags "$TAG" || { echo "Build failed for $component_dir"; exit 1; }
    output_name="${output_name}-${TAG}"  # 在 zip 文件名中加上 tag
  else
    go build -trimpath || { echo "Build failed for $component_dir"; exit 1; }
  fi
  
  # 打包为 ZIP 文件
  zip "${output_name}.zip" "./$(basename "$component_dir")" || { echo "Zipping failed for $component_dir"; exit 1; }
  
  # 移动 ZIP 文件到 release 目录
  mv "${output_name}.zip" "../../$RELEASE_DIR/" || { echo "Move failed for $component_dir"; exit 1; }
  
  # 删除二进制文件
  rm "./$(basename "$component_dir")" || { echo "Cleanup failed for $component_dir"; exit 1; }
  
  # 返回项目根目录
  cd - > /dev/null || exit 1
}

# 编译并打包 agent
build_and_package "$AGENT_DIR" "agent-linux"

# 编译并打包 controller
build_and_package "$CONTROLLER_DIR" "controller-linux"

# 计算并显示 MD5 校验值
md5sum "$RELEASE_DIR/agent-linux.zip"
md5sum "$RELEASE_DIR/controller-linux.zip"
md5sum "$RELEASE_DIR/agent-linux-box.zip"
md5sum "$RELEASE_DIR/controller-linux-box.zip"
