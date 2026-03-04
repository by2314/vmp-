# VMPacker Android

VMPacker 的 Android 前端——使用 [Fyne](https://fyne.io/) 实现的极简 GUI，通过 root shell 调用 vmpacker CLI，对 Android 设备上的 ARM64 ELF 文件（如 .so 或可执行文件）进行 VMP 保护。

## 架构说明

```
android/
├── main.go                  # Fyne GUI 主程序
├── go.mod                   # Go 模块（依赖 fyne.io/fyne/v2）
├── build.sh                 # 构建脚本（编译 CLI + 打包 APK）
├── vmpacker_android_arm64   # 占位符，由 build.sh 生成真实二进制
└── README.md                # 本文档
```

## 前置条件

### 宿主机（构建环境）

| 工具 | 版本 | 说明 |
|------|------|------|
| Go | 1.21+ | https://go.dev/dl/ |
| Android NDK | r25+ | https://developer.android.com/ndk/downloads |
| Android SDK | build-tools 30+ | 提供 `apksigner` / `aapt` |
| fyne CLI | latest | `go install fyne.io/fyne/v2/cmd/fyne@latest` |
| JDK | 11+ | https://adoptium.net/ |

### 目标 Android 设备

- **架构**：arm64（AArch64）
- **root 权限**：已通过 Magisk 或 SuperSU 获取
- **Android 版本**：5.0+（API 21+）

## 构建步骤

```bash
cd android/
export ANDROID_NDK_HOME=/path/to/android-ndk-r25c
export ANDROID_HOME=/path/to/android-sdk
chmod +x build.sh
./build.sh
```

## 使用方式

1. 打开 **VMPacker** 应用
2. 输入目标 ELF 文件路径，例：`/data/local/tmp/libtarget.so`
3. 填写保护函数名（逗号分隔，可留空）
4. 点击 **🛡️ 开始保护**
5. 查看日志，输出文件为原路径 + `.vmp`