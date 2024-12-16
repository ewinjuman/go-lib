package utils

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var gormSourceDir string

func init() {
	_, file, _, _ := runtime.Caller(0)
	// get go-lib source directory
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

// FileWithLineNum return the file name and line number of the current file
func FileWithLineNum() string {
	// the second caller usually from internal, so set i start from 2
	for i := 2; i < 15; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok && (!strings.HasPrefix(file, gormSourceDir) || strings.HasSuffix(file, "_test.go")) &&
			!strings.HasSuffix(file, ".gen.go") {
			return file + ":" + strconv.FormatInt(int64(line), 10)
		}
	}

	return ""
}
