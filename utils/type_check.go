package utils

import "reflect"

func IsError(maybeErr interface{}) (bool, interface{}) {

	if maybeErr == nil {
		return false, nil
	}

	if reflect.TypeOf(maybeErr).Name() == "Value" {
		return IsError(maybeErr.(reflect.Value).Interface())
	}

	if reflect.ValueOf(maybeErr).MethodByName("Error").Kind() != reflect.Func {
		return false, maybeErr
	}

	return true, maybeErr
}

// IsFunc check if args is func kind
func IsFunc(maybeFunc interface{}) bool {
	if maybeFunc != nil {
		return reflect.TypeOf(maybeFunc).Kind() == reflect.Func
	}
	return false
}

// IsStruct check if arg is struct kind
func IsStruct(maybeStruct interface{}) bool {
	if maybeStruct != nil {
		if IsPtr(maybeStruct) {
			refVal := reflect.ValueOf(maybeStruct)
			return reflect.Indirect(refVal).Kind() == reflect.Struct
		}
		return reflect.TypeOf(maybeStruct).Kind() == reflect.Struct
	}
	return false
}

// IsPtr check if arg is pointer kind
func IsPtr(maybePtr interface{}) bool {
	if maybePtr != nil {
		if rf, ok := maybePtr.(reflect.Value); ok {
			if !rf.IsValid() {
				return false
			}
			return IsPtr(rf.Interface())
		}
		return reflect.TypeOf(maybePtr).Kind() == reflect.Ptr
	}
	return false
}

// IsMap check if arg is map kind
func IsMap(maybeMap interface{}) bool {
	if maybeMap != nil {
		if rf, ok := maybeMap.(reflect.Value); ok {
			if !rf.IsValid() {
				return false
			}
			return IsMap(rf.Interface())
		}

		if IsPtr(maybeMap) {
			refVal := reflect.ValueOf(maybeMap)
			return reflect.Indirect(refVal).Kind() == reflect.Map
		}
		return reflect.TypeOf(maybeMap).Kind() == reflect.Map
	}
	return false
}

// IsSlice check if arg is slice kind
func IsSlice(maybeSlice interface{}) bool {
	if maybeSlice != nil {
		if rf, ok := maybeSlice.(reflect.Value); ok {
			if !rf.IsValid() {
				return false
			}
			return IsSlice(rf.Interface())
		}

		if IsPtr(maybeSlice) {
			refVal := reflect.ValueOf(maybeSlice)
			return reflect.Indirect(refVal).Kind() == reflect.Slice
		}
		return reflect.TypeOf(maybeSlice).Kind() == reflect.Slice
	}
	return false
}

// IsCompound ...
func IsCompound(maybeCompound interface{}) bool {
	return IsMap(maybeCompound) ||
		IsSlice(maybeCompound) ||
		IsStruct(maybeCompound)
}

func IsEmpty(arg interface{}) bool {

	if rf, ok := arg.(reflect.Value); ok {
		if !rf.IsValid() {
			return true
		}
		return IsEmpty(rf.Interface())
	}

	if arg != nil {
		switch {
		case IsMap(arg), IsSlice(arg):
			return reflect.ValueOf(arg).Len() < 1
		case IsStruct(arg):
			return reflect.Indirect(reflect.ValueOf(arg)).IsZero()
		default:
			argType := reflect.TypeOf(arg)
			zeroVal := reflect.Zero(argType).Interface()
			return reflect.DeepEqual(arg, zeroVal)
		}
	}
	return true
}
