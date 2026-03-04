# 🛡️ vmp-

ARM64 ELF Virtual Machine Protection System

> 本仓库由 VMPacker 迁移而来，包含 Android 前端及核心保护引擎。

## 项目结构

```
vmp-/
├── go.mod                   # Go 根模块
├── android/                 # Android Fyne GUI 前端
│   ├── main.go
│   ├── go.mod
│   └── README.md
└── vmp-gui/                 # Wails 桌面 GUI
    └── go.mod
```

## 快速开始

请参考 [android/README.md](android/README.md) 了解 Android 客户端构建方式。