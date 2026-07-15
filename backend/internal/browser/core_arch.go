package browser

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"os"
	"runtime"
)

func validateExecutableArchitecture(path string) error {
	if path == "" {
		return fmt.Errorf("未找到浏览器可执行文件")
	}
	switch runtime.GOOS {
	case "darwin":
		if fat, err := macho.OpenFat(path); err == nil {
			defer fat.Close()
			for _, arch := range fat.Arches {
				if machoCPUCompatible(arch.Cpu) {
					return ensureExecutable(path)
				}
			}
			return fmt.Errorf("Mach-O 不包含当前 %s 架构", runtime.GOARCH)
		}
		file, err := macho.Open(path)
		if err != nil {
			return fmt.Errorf("不是有效的 Mach-O: %w", err)
		}
		defer file.Close()
		if !machoCPUCompatible(file.Cpu) {
			return fmt.Errorf("Mach-O 架构 %s 与宿主 %s 不兼容", file.Cpu.String(), runtime.GOARCH)
		}
		return ensureExecutable(path)
	case "linux":
		file, err := elf.Open(path)
		if err != nil {
			return fmt.Errorf("不是有效的 ELF: %w", err)
		}
		defer file.Close()
		if (runtime.GOARCH == "amd64" && file.Machine != elf.EM_X86_64) || (runtime.GOARCH == "arm64" && file.Machine != elf.EM_AARCH64) {
			return fmt.Errorf("ELF 架构 %s 与宿主 %s 不兼容", file.Machine.String(), runtime.GOARCH)
		}
		return ensureExecutable(path)
	case "windows":
		file, err := pe.Open(path)
		if err != nil {
			return fmt.Errorf("不是有效的 PE: %w", err)
		}
		defer file.Close()
		if (runtime.GOARCH == "amd64" && file.Machine != pe.IMAGE_FILE_MACHINE_AMD64) || (runtime.GOARCH == "arm64" && file.Machine != pe.IMAGE_FILE_MACHINE_ARM64) {
			return fmt.Errorf("PE 架构与宿主 %s 不兼容", runtime.GOARCH)
		}
	}
	return nil
}

func machoCPUCompatible(cpu macho.Cpu) bool {
	return (runtime.GOARCH == "amd64" && cpu == macho.CpuAmd64) || (runtime.GOARCH == "arm64" && cpu == macho.CpuArm64)
}
func ensureExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := info.Mode()
	if mode&0111 == 0 {
		if err := os.Chmod(path, mode|0755); err != nil {
			return fmt.Errorf("恢复可执行权限失败: %w", err)
		}
	}
	return nil
}
