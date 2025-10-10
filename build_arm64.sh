#!/bin/bash

# 相对路径定义
RELEASE_DIR="script/bin"
AGENT_DIR="cmd/agent"
CONTROLLER_DIR="cmd/controller"

build_and_package() {
  local component_dir=$1
  local output_name=$2

  cd "$component_dir" || exit 1
  
  # 编译二进制文件
  GOOS=linux GOARCH=arm64 go build -trimpath|| { echo "Build failed for $component_dir"; exit 1; }
  
  # 打包为 ZIP 文件
  zip "${output_name}.zip" "./$(basename "$component_dir")" || { echo "Zipping failed for $component_dir"; exit 1; }
  
  # 移动 ZIP 文件到 release 目录
  mv "${output_name}.zip" "../../script/bin/" || { echo "Move failed for $component_dir"; exit 1; }
  
  # 删除二进制文件
  rm "./$(basename "$component_dir")" || { echo "Cleanup failed for $component_dir"; exit 1; }
   
  # 返回项目根目录
  cd - > /dev/null || exit 1
}

# 编译并打包 agent
build_and_package "$AGENT_DIR" "agent-arm64"

# 编译并打包 controller
build_and_package "$CONTROLLER_DIR" "controller-arm64"

# 计算并显示 MD5 校验值
md5sum "$RELEASE_DIR/agent-arm64.zip"
md5sum "$RELEASE_DIR/controller-arm64.zip"
