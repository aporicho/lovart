package selfmanage

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func platformAsset(goos string, goarch string) (string, error) {
	switch goos + "/" + goarch {
	case "darwin/arm64":
		return "lovart-macos-arm64", nil
	case "linux/amd64":
		return "lovart-linux-x64", nil
	case "windows/amd64":
		return "lovart-windows-x64.exe", nil
	default:
		return "", inputError("unsupported upgrade platform", map[string]any{
			"goos":   goos,
			"goarch": goarch,
		})
	}
}

func checksumForAsset(sumsPath string, assetName string) (string, error) {
	file, err := os.Open(sumsPath)
	if err != nil {
		return "", internalError("open SHA256SUMS", map[string]any{"path": sumsPath, "error": err.Error()})
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == assetName {
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", internalError("read SHA256SUMS", map[string]any{"path": sumsPath, "error": err.Error()})
	}
	return "", inputError("SHA256SUMS does not contain asset", map[string]any{"asset": assetName})
}

func verifySHA256(path string, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return internalError("open asset for checksum", map[string]any{"path": path, "error": err.Error()})
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return internalError("hash release asset", map[string]any{"path": path, "error": err.Error()})
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return inputError("checksum mismatch for release asset", map[string]any{
			"path":     path,
			"expected": expected,
			"actual":   actual,
		})
	}
	return nil
}

func copyFile(source string, dest string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return internalError("open source file", map[string]any{"path": source, "error": err.Error()})
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return internalError("create target directory", map[string]any{"path": filepath.Dir(dest), "error": err.Error()})
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return internalError("create target file", map[string]any{"path": dest, "error": err.Error()})
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return internalError("copy target file", map[string]any{"path": dest, "error": err.Error()})
	}
	if err := out.Close(); err != nil {
		return internalError("close target file", map[string]any{"path": dest, "error": err.Error()})
	}
	return nil
}

func installBinary(source string, target string) error {
	temp := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".upgrade.tmp")
	_ = os.Remove(temp)
	if err := copyFile(source, temp, 0755); err != nil {
		return err
	}
	if err := os.Chmod(temp, 0755); err != nil {
		_ = os.Remove(temp)
		return internalError("mark upgraded binary executable", map[string]any{"path": temp, "error": err.Error()})
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return internalError("replace lovart binary", map[string]any{"path": target, "error": err.Error()})
	}
	return nil
}

func unzipFile(zipPath string, dest string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return internalError("open extension zip", map[string]any{"path": zipPath, "error": err.Error()})
	}
	defer reader.Close()
	cleanDest, err := filepath.Abs(dest)
	if err != nil {
		return internalError("resolve extension unzip path", map[string]any{"path": dest, "error": err.Error()})
	}
	for _, file := range reader.File {
		target := filepath.Join(cleanDest, file.Name)
		cleanTarget, err := filepath.Abs(target)
		if err != nil {
			return internalError("resolve extension zip entry", map[string]any{"path": target, "error": err.Error()})
		}
		if cleanTarget != cleanDest && !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) {
			return inputError("extension zip contains unsafe path", map[string]any{"entry": file.Name})
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTarget, file.Mode()); err != nil {
				return internalError("create extension directory", map[string]any{"path": cleanTarget, "error": err.Error()})
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
			return internalError("create extension parent directory", map[string]any{"path": filepath.Dir(cleanTarget), "error": err.Error()})
		}
		in, err := file.Open()
		if err != nil {
			return internalError("open extension zip entry", map[string]any{"entry": file.Name, "error": err.Error()})
		}
		out, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			_ = in.Close()
			return internalError("write extension zip entry", map[string]any{"path": cleanTarget, "error": err.Error()})
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return internalError("copy extension zip entry", map[string]any{"path": cleanTarget, "error": copyErr.Error()})
		}
		if closeErr != nil {
			return internalError("close extension zip entry", map[string]any{"path": cleanTarget, "error": closeErr.Error()})
		}
	}
	return nil
}

func replaceExtension(zipPath string, targetDir string, stagingDir string) error {
	unpacked := filepath.Join(stagingDir, "extension-unpacked")
	if err := os.RemoveAll(unpacked); err != nil {
		return internalError("clear extension staging directory", map[string]any{"path": unpacked, "error": err.Error()})
	}
	if err := unzipFile(zipPath, unpacked); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return internalError("create extension parent directory", map[string]any{"path": filepath.Dir(targetDir), "error": err.Error()})
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return internalError("remove old extension directory", map[string]any{"path": targetDir, "error": err.Error()})
	}
	if err := os.Rename(unpacked, targetDir); err != nil {
		return internalError("replace extension directory", map[string]any{"path": targetDir, "error": err.Error()})
	}
	return nil
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func normalizePath(path string) (string, error) {
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		return "", inputError("resolve path", map[string]any{"path": path, "error": err.Error()})
	}
	return abs, nil
}

func executablePath(explicit string, fallback string) (string, bool, error) {
	if explicit != "" {
		path, err := normalizePath(explicit)
		return path, true, err
	}
	if fallback == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", false, internalError("resolve current executable", map[string]any{"error": err.Error()})
		}
		fallback = exe
	}
	path, err := normalizePath(fallback)
	if err != nil {
		return "", false, err
	}
	return path, false, nil
}

func looksLikeGoRun(path string) bool {
	return strings.Contains(path, "go-build") && strings.Contains(path, string(os.PathSeparator)+"exe"+string(os.PathSeparator))
}

func runtimePlatform(goos string, goarch string) (string, string) {
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return goos, goarch
}
