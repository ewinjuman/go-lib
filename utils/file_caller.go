package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var (
	gormSourceDir string
	moduleRoots   sync.Map // dir string → module root string (cached)
)

func init() {
	_, file, _, _ := runtime.Caller(0)
	gormSourceDir = sourceDir(file)
}

func sourceDir(file string) string {
	dir := filepath.Dir(file)
	dir = filepath.Dir(dir)

	s := filepath.Dir(dir)
	if filepath.Base(s) != "go-lib" {
		s = dir
	}
	return filepath.ToSlash(s) + "/"
}

// findModuleRoot walks up from dir until it finds a directory containing go.mod.
// Results are cached per directory so the filesystem is only touched once per path.
func findModuleRoot(dir string) string {
	if v, ok := moduleRoots.Load(dir); ok {
		return v.(string)
	}

	current := dir
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			root := filepath.ToSlash(current) + "/"
			moduleRoots.Store(dir, root)
			return root
		}
		parent := filepath.Dir(current)
		if parent == current {
			moduleRoots.Store(dir, "")
			return ""
		}
		current = parent
	}
}

// FileWithLineNum returns the file path relative to the nearest go.mod root and line number.
func FileWithLineNum() string {
	for i := 2; i < 15; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		if (!strings.HasPrefix(file, gormSourceDir) || strings.HasSuffix(file, "_test.go")) &&
			!strings.HasSuffix(file, ".gen.go") {

			file = filepath.ToSlash(file)

			if root := findModuleRoot(filepath.Dir(filepath.FromSlash(file))); root != "" {
				if strings.HasPrefix(file, root) {
					file = file[len(root):]
				}
			}

			return file + ":" + strconv.FormatInt(int64(line), 10)
		}
	}
	return ""
}
