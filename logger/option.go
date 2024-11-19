package logger

import "time"

type Options struct {
	FileLocation       string        `json:"fileLocation"`
	FileName           string        `json:"fileName"`
	FileMaxAge         time.Duration `json:"fileMaxAge"`
	Stdout             bool          `json:"stdout"`
	MaskingLogJsonPath string        `json:"maskingLogJsonPath"`
}
