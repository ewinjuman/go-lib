package cache

type Policy func(key string) bool

func PolicyAll() Policy {
	return func(string) bool { return true }
}

func PolicyNone() Policy {
	return func(string) bool { return false }
}

func PolicyKeys(keys ...string) Policy {
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}
	return func(key string) bool {
		_, ok := set[key]
		return ok
	}
}

func PolicyFunc(fn func(string) bool) Policy {
	return fn
}
