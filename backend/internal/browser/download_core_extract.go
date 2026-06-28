package browser

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

type archiveEntry struct {
	Name     string
	Mode     os.FileMode
	Dir      bool
	Open     func() (io.ReadCloser, error)
	LinkName string
	Symlink  bool
}

func SupportedCoreArchivePattern() string {
	return "*.zip;*.tar;*.tar.gz;*.tgz;*.tar.xz;*.txz;*.tar.bz2;*.tbz2"
}

func SupportedCoreArchiveDescription() string {
	return "支持 ZIP、TAR、TAR.GZ、TAR.XZ、TAR.BZ2"
}

func coreArchiveTempPattern(rawURL string) string {
	lowerName := strings.ToLower(strings.TrimSpace(rawURL))
	if parsed, err := filepathFromURLPath(lowerName); err == nil && parsed != "" {
		lowerName = parsed
	}
	suffixes := []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".txz", ".tbz2", ".zip", ".tar"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(lowerName, suffix) {
			return "download_*" + suffix
		}
	}
	return "download_*"
}

func filepathFromURLPath(raw string) (string, error) {
	parts := strings.SplitN(raw, "?", 2)
	parts = strings.SplitN(parts[0], "#", 2)
	return filepath.Base(parts[0]), nil
}

func extractCoreArchiveAndStripRoot(archivePath, dest string, progressCb func(int, string)) error {
	entries, closeEntries, err := openCoreArchiveEntries(archivePath)
	if err != nil {
		return err
	}
	defer closeEntries()

	if len(entries) == 0 {
		return fmt.Errorf("空的压缩包")
	}

	rootPrefix, hasCommonRoot := detectCommonArchiveRoot(entries)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	for index, entry := range entries {
		percent := int((float64(index) / float64(len(entries))) * 100)
		if index%50 == 0 {
			progressCb(percent, fmt.Sprintf("正在解压文件 %d / %d...", index+1, len(entries)))
		}

		cleanName := normalizeArchiveEntryName(entry.Name)
		if hasCommonRoot {
			if cleanName == rootPrefix || cleanName == strings.TrimSuffix(rootPrefix, "/") {
				continue
			}
			cleanName = strings.TrimPrefix(cleanName, rootPrefix)
		}
		if cleanName == "" || cleanName == "/" {
			continue
		}

		targetPath, err := safeArchiveTargetPath(dest, cleanName)
		if err != nil {
			return err
		}
		if entry.Dir {
			if err := os.MkdirAll(targetPath, entry.Mode.Perm()); err != nil {
				return err
			}
			continue
		}
		if entry.Symlink {
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			_ = os.Remove(targetPath)
			if err := os.Symlink(entry.LinkName, targetPath); err != nil {
				return fmt.Errorf("创建符号链接失败 %s: %w", cleanName, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := writeArchiveEntryFile(targetPath, entry); err != nil {
			return err
		}
	}

	progressCb(100, "解压完成！")
	return nil
}

func ExtractCoreArchiveAndStripRootForImport(archivePath, dest string) error {
	return extractCoreArchiveAndStripRoot(archivePath, dest, func(int, string) {})
}

func openCoreArchiveEntries(archivePath string) ([]archiveEntry, func(), error) {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		return openZipArchiveEntries(archivePath)
	}
	if isTarArchivePath(lower) {
		return openTarArchiveEntries(archivePath)
	}
	if entries, closeEntries, err := openZipArchiveEntries(archivePath); err == nil {
		return entries, closeEntries, nil
	}
	return openTarArchiveEntries(archivePath)
}

func openZipArchiveEntries(archivePath string) ([]archiveEntry, func(), error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, func() {}, err
	}
	entries := make([]archiveEntry, 0, len(reader.File))
	for _, file := range reader.File {
		zipFile := file
		entries = append(entries, archiveEntry{
			Name: zipFile.Name,
			Mode: zipFile.Mode(),
			Dir:  zipFile.FileInfo().IsDir(),
			Open: func() (io.ReadCloser, error) {
				return zipFile.Open()
			},
		})
	}
	return entries, func() { _ = reader.Close() }, nil
}

func openTarArchiveEntries(archivePath string) ([]archiveEntry, func(), error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, func() {}, err
	}
	reader, err := tarStreamReader(archivePath, file)
	if err != nil {
		_ = file.Close()
		return nil, func() {}, err
	}

	tmpDir, err := os.MkdirTemp(filepath.Dir(archivePath), "archive_entries_*")
	if err != nil {
		_ = file.Close()
		return nil, func() {}, err
	}
	cleanup := func() {
		_ = file.Close()
		_ = os.RemoveAll(tmpDir)
	}

	tarReader := tar.NewReader(reader)
	var entries []archiveEntry
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}

		mode := header.FileInfo().Mode()
		entry := archiveEntry{
			Name:     header.Name,
			Mode:     mode,
			Dir:      header.FileInfo().IsDir(),
			LinkName: header.Linkname,
			Symlink:  header.Typeflag == tar.TypeSymlink,
		}
		if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
			spoolPath := filepath.Join(tmpDir, fmt.Sprintf("entry_%06d", len(entries)))
			out, err := os.OpenFile(spoolPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
			if err != nil {
				cleanup()
				return nil, func() {}, err
			}
			_, copyErr := io.Copy(out, tarReader)
			closeErr := out.Close()
			if copyErr != nil {
				cleanup()
				return nil, func() {}, copyErr
			}
			if closeErr != nil {
				cleanup()
				return nil, func() {}, closeErr
			}
			entry.Open = func() (io.ReadCloser, error) {
				return os.Open(spoolPath)
			}
		}
		entries = append(entries, entry)
	}
	return entries, cleanup, nil
}

func tarStreamReader(archivePath string, file *os.File) (io.Reader, error) {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return gzip.NewReader(file)
	case strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz"):
		return xz.NewReader(file)
	case strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz2"):
		return bzip2.NewReader(file), nil
	case strings.HasSuffix(lower, ".tar"):
		return file, nil
	default:
		return file, nil
	}
}

func isTarArchivePath(path string) bool {
	for _, suffix := range []string{".tar", ".tar.gz", ".tgz", ".tar.xz", ".txz", ".tar.bz2", ".tbz2"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func detectCommonArchiveRoot(entries []archiveEntry) (string, bool) {
	var rootPrefix string
	for _, entry := range entries {
		cleanName := normalizeArchiveEntryName(entry.Name)
		parts := strings.SplitN(cleanName, "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		if rootPrefix == "" {
			rootPrefix = parts[0] + "/"
			continue
		}
		if !strings.HasPrefix(cleanName, rootPrefix) && cleanName != strings.TrimSuffix(rootPrefix, "/") {
			return "", false
		}
	}
	return rootPrefix, rootPrefix != ""
}

func normalizeArchiveEntryName(name string) string {
	cleanName := filepath.ToSlash(strings.TrimSpace(name))
	cleanName = strings.TrimPrefix(cleanName, "/")
	return filepath.ToSlash(filepath.Clean(cleanName))
}

func safeArchiveTargetPath(dest, cleanName string) (string, error) {
	if cleanName == "." || strings.HasPrefix(cleanName, "../") || cleanName == ".." || filepath.IsAbs(cleanName) {
		return "", fmt.Errorf("非法文件路径: %s", cleanName)
	}
	targetPath := filepath.Join(dest, filepath.FromSlash(cleanName))
	destClean := filepath.Clean(dest)
	targetClean := filepath.Clean(targetPath)
	if targetClean != destClean && !strings.HasPrefix(targetClean, destClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("非法文件路径: %s", cleanName)
	}
	return targetPath, nil
}

func writeArchiveEntryFile(targetPath string, entry archiveEntry) error {
	if entry.Open == nil {
		return nil
	}
	outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, entry.Mode.Perm())
	if err != nil {
		return fmt.Errorf("打开解压文件写入失败 %s: %w", targetPath, err)
	}
	rc, err := entry.Open()
	if err != nil {
		_ = outFile.Close()
		return fmt.Errorf("读取压缩包文件失败 %s: %w", entry.Name, err)
	}
	_, copyErr := io.Copy(outFile, rc)
	closeReadErr := rc.Close()
	closeWriteErr := outFile.Close()
	if copyErr != nil {
		return fmt.Errorf("写入文件流失败 %s: %w", targetPath, copyErr)
	}
	if closeReadErr != nil {
		return closeReadErr
	}
	return closeWriteErr
}
