package extra

// GetOr safely retrieves a value from a map, asserts it to type V,
// or returns the fallback if the key is missing or the type doesn't match.
func GetOr[K comparable, V any](hatch map[K]any, key K, def V) V {
	v, exists := hatch[key]
	if !exists {
		return def
	}

	if v, ok := v.(V); ok {
		return v
	}

	return def
}

func GetOrDefault[K comparable, V any](hatch map[K]any, key K) V {
	var zero V
	return GetOr(hatch, key, zero)
}
