package dbsql

import (
	"encoding/json"
	"sort"
)

func MarshalLabelItems[T any](labels []T) ([]byte, error) {
	if labels == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(labels)
}

func UnmarshalLabelItems[T any](raw []byte, build func(key, value string) T, keyOf func(T) string) ([]T, error) {
	var labels []T
	if err := json.Unmarshal(raw, &labels); err == nil {
		return labels, nil
	}

	var legacy map[string]string
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, err
	}

	labels = make([]T, 0, len(legacy))
	for key, value := range legacy {
		labels = append(labels, build(key, value))
	}
	sort.Slice(labels, func(i, j int) bool {
		return keyOf(labels[i]) < keyOf(labels[j])
	})
	return labels, nil
}
