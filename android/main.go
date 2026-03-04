// VMPacker Android GUI
//
// 这是 VMPacker 的 Android 前端，使用 Fyne 实现极简图形界面。
//
// 主流程：
//  1. 用户在输入框中输入（或粘贴）目标 ELF 文件的绝对路径。
//  2. 可选填要保护的函数名（逗号分隔）。
//  3. 点击"开始保护"按钮后，后台通过 su -c 以 root 权限调用已部署到
//     /data/local/tmp/vmpacker 的 CLI 工具，对目标 ELF 执行保护。
//  4. 执行输出（日志）回传到界面底部的日志区域。
//
// 内嵌二进制说明：
//   - vmpacker_android_arm64：由 build.sh 交叉编译生成后置于本目录，
//     通过 //go:embed 嵌入 APK。APK 在保护前将其写入
//     /data/local/tmp/vmpacker 并 chmod +x 赋予执行权限。
//   - stub / vm_interp.bin：vmpacker CLI 在编译时已内嵌，无需单独分发。
//
// 后续扩展提示（搜索 "TODO(extend)" 标记）：
//   - TODO(extend): 改用 dialog.ShowFileOpen 文件选择对话框
//   - TODO(extend): 增加输出路径输入框
//   - TODO(extend): 暴露 -addr、-strip、-token、-debug 等高级选项
//   - TODO(extend): 实时流式读取子进程输出

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	// 嵌入 vmpacker Android arm64 CLI 可执行文件。
	// 构建前请先运行 build.sh 生成该文件。
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// vmpackerBin 是通过 go:embed 嵌入的 vmpacker Android arm64 CLI 二进制。
// build.sh 负责在打包前编译并输出为本目录的 vmpacker_android_arm64。
//
// 注意：仓库中的 vmpacker_android_arm64 是一个占位脚本。
// 正式使用前必须先运行 build.sh 生成真实的 arm64 二进制，
// 否则嵌入的将是占位脚本，设备上执行时会报错并退出。
//
//go:embed vmpacker_android_arm64
var vmpackerBin []byte

// vmpackerPath 是 vmpacker 在设备上的工作路径。
const vmpackerPath = "/data/local/tmp/vmpacker"

// suTimeout 是每条 root shell 命令的最长等待时间。
const suTimeout = 180 * time.Second

func main() {
	// ── 初始化 Fyne 应用 ─────────────────────────────────────
	a := app.New()
	w := a.NewWindow("VMPacker")
	w.Resize(fyne.NewSize(480, 700))

	// ── UI 组件 ──────────────────────────────────────────────

	// ELF 文件路径输入框
	// TODO(extend): 替换为 dialog.ShowFileOpen 文件选择对话框
	elfEntry := widget.NewEntry()
	elfEntry.SetPlaceHolder("/data/local/tmp/target.elf")

	// 函数名输入框（逗号分隔，留空则跳过 -func 参数）
	// TODO(extend): 增加 -addr 地址范围输入框
	funcEntry := widget.NewEntry()
	funcEntry.SetPlaceHolder("check_license,verify_token（留空则不按函数名保护）")

	// 日志输出区域（只读多行文本）
	logText := widget.NewMultiLineEntry()
	logText.Disable()
	logText.SetMinRowsVisible(12)
	logText.SetPlaceHolder("保护日志将显示在此处…")

	// 无限进度条（保护运行期间显示）
	progress := widget.NewProgressBarInfinite()
	progress.Hide()

	// appendLog 向日志框追加一行文本。
	appendLog := func(line string) {
		logText.Enable()
		cur := logText.Text
		if cur == "" {
			logText.SetText(line)
		} else {
			logText.SetText(cur + "\n" + line)
		}
		logText.Disable()
	}

	// ── 保护按钮逻辑 ─────────────────────────────────────────

	var protectBtn *widget.Button
	protectBtn = widget.NewButton("🛡️ 开始保护", func() {
		elfPath := strings.TrimSpace(elfEntry.Text)
		if elfPath == "" {
			dialog.ShowError(fmt.Errorf("请输入目标 ELF 文件路径"), w)
			return
		}

		// 保护期间禁用按钮，防止重复触发
		protectBtn.Disable()
		progress.Show()
		logText.Enable()
		logText.SetText("")
		logText.Disable()

		// 在独立 goroutine 中执行，避免阻塞 Fyne UI 线程
		go func() {
			defer func() {
				progress.Hide()
				protectBtn.Enable()
			}()

			// ── 步骤 1: 将 vmpacker 部署到设备 ────────────────
			appendLog("[*] 正在将 vmpacker 部署到设备…")
			if err := deployVMPacker(appendLog); err != nil {
				appendLog(fmt.Sprintf("[!] 部署失败: %v", err))
				return
			}
			appendLog("[+] vmpacker 已就绪: " + vmpackerPath)

			// ── 步骤 2: 构造并执行保护命令 ────────────────────
			outPath := elfPath + ".vmp"
			// TODO(extend): 从单独的输出路径输入框读取 outPath

			cmdArgs := buildCmdArgs(elfPath, outPath, strings.TrimSpace(funcEntry.Text))
			shellCmd := vmpackerPath + " " + cmdArgs
			appendLog(fmt.Sprintf("[*] 执行: su -c '%s'", shellCmd))

			output, err := runAsRoot(shellCmd)
			if strings.TrimSpace(output) != "" {
				appendLog(output)
			}
			if err != nil {
				appendLog(fmt.Sprintf("[!] 保护失败: %v", err))
				return
			}
			appendLog(fmt.Sprintf("[✓] 保护完成! 输出文件: %s", outPath))
			}()
	})

	// ── 布局 ────────────────────────────────────────────────

	content := container.NewVBox(
		widget.NewLabelWithStyle(
			"VMPacker — ARM64 ELF 保护工具",
			fyne.TextAlignCenter,
			fyne.TextStyle{Bold: true},
		),
		widget.NewSeparator(),
		widget.NewLabel("目标 ELF 文件路径:"),
		elfEntry,
		widget.NewLabel("保护函数名（逗号分隔，可留空）:"),
		funcEntry,
		protectBtn,
		progress,
		widget.NewSeparator(),
		widget.NewLabel("执行日志:"),
		container.NewScroll(logText),
	)

	w.SetContent(content)
	w.ShowAndRun()
}

// deployVMPacker 将内嵌的 vmpacker 二进制部署到设备上。
func deployVMPacker(log func(string)) error {
	if err := os.WriteFile(vmpackerPath, vmpackerBin, 0700); err == nil {
		return nil
	}
	log("[~] 直接写入失败，改用 root shell 部署…")
	return deployViaRootShell(log)
}

// deployViaRootShell 通过 base64 编码 + root shell 将 vmpacker 写入设备。
func deployViaRootShell(log func(string)) error {
	const chunkBytes = 512

	clearCmd := fmt.Sprintf("truncate -s 0 %s 2>/dev/null || printf '' > %s", shellQuote(vmpackerPath), shellQuote(vmpackerPath))
	if out, err := runAsRoot(clearCmd); err != nil {
		return fmt.Errorf("创建目标文件失败 (%w): %s", err, out)
	}

	total := len(vmpackerBin)
	blocks := (total + chunkBytes - 1) / chunkBytes
	log(fmt.Sprintf("[~] base64 分块写入 (%d 块 / %d 字节)…", blocks, total))

	for i := 0; i < total; i += chunkBytes {
		end := i + chunkBytes
		if end > total {
			end = total
		}
		chunk := base64.StdEncoding.EncodeToString(vmpackerBin[i:end])
		cmd := fmt.Sprintf("echo '%s' | base64 -d >> %s", chunk, shellQuote(vmpackerPath))
		if out, err := runAsRoot(cmd); err != nil {
			return fmt.Errorf("写入第 %d 块失败 (%w): %s", i/chunkBytes, err, out)
		}
	}

	if out, err := runAsRoot(fmt.Sprintf("chmod 700 %s", shellQuote(vmpackerPath))); err != nil {
		return fmt.Errorf("chmod 700 失败 (%w): %s", err, out)
	}
	return nil
}

// buildCmdArgs 根据输入参数构造 vmpacker 的命令行参数字符串。
// TODO(extend): 增加 -addr、-strip、-token、-debug 等参数。
func buildCmdArgs(elfPath, outPath, funcNames string) string {
	args := make([]string, 0, 6)
	if funcNames != "" {
		args = append(args, "-func", shellQuote(funcNames))
	}
	args = append(args, "-o", shellQuote(outPath))
	args = append(args, shellQuote(elfPath))
	return strings.Join(args, " ")
}

// shellQuote 为 shell 参数添加单引号转义。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `\'`)+ "'"
}

// runAsRoot 通过 su -c 以 root 权限执行 shell 命令。
func runAsRoot(shellCmd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), suTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "su", "-c", shellCmd)
	out, err := cmd.CombinedOutput()
	return string(out), err
}