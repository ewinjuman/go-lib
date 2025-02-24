package utils

import (
	"strings"
)

func ContainsInArr(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func ContainsInArrNoCaseSensitive(s []string, e string) bool {
	for _, a := range s {
		if strings.ToLower(a) == strings.ToLower(e) {
			return true
		}
	}
	return false
}

// Get string between two string
func BetweenString(value string, after string, before string) string {
	// Get substring between two strings.
	posFirst := strings.Index(value, after)
	if posFirst == -1 {
		return ""
	}
	posLast := strings.Index(value, before)
	if posLast == -1 {
		return ""
	}
	posFirstAdjusted := posFirst + len(after)
	if posFirstAdjusted >= posLast {
		return ""
	}
	return value[posFirstAdjusted:posLast]
}

// BeforeString Get substring before a string.
func BeforeString(value string, before string) string {
	pos := strings.Index(value, before)
	if pos == -1 {
		return ""
	}
	return value[0:pos]
}

// AfterString Get substring after a string.
func AfterString(value string, after string) string {
	pos := strings.LastIndex(value, after)
	if pos == -1 {
		return ""
	}
	adjustedPos := pos + len(after)
	if adjustedPos >= len(value) {
		return ""
	}
	return value[adjustedPos:]
}

// Or check if testValue is empty, if empty return defaultValue.
func Or(testValue, defaultValue interface{}) interface{} {
	if !IsEmpty(testValue) {
		return testValue
	}
	return defaultValue
}

// Must is a helper function that takes a value of any type and an error.
// // If the error is nil, it returns the value; if the error is non-nil, it panics.
func Must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

// delete the pointer from value
func SafeDeref[T any](p *T) T {
	if p == nil {
		var v T
		return v
	}
	return *p
}
