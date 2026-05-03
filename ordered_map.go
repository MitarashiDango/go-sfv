package sfv

import "slices"

// OrderedMap - キー挿入順を保持する map。同一キーへの Set は順序を維持し値のみ更新する。
type OrderedMap[T any] struct {
	keys   []string
	values map[string]T
}

// Keys - 挿入順のキー列を返す。空の場合は nil ではなく空 slice。
func (p *OrderedMap[T]) Keys() []string {
	if len(p.keys) == 0 {
		return []string{}
	}

	return p.keys
}

func (p *OrderedMap[T]) Len() int {
	return len(p.keys)
}

func (p *OrderedMap[T]) Has(key string) bool {
	return slices.Contains(p.keys, key)
}

func (p *OrderedMap[T]) Get(key string) any {
	if !p.Has(key) {
		return nil
	}

	v, exists := p.values[key]
	if !exists {
		return nil
	}

	return v
}

func (p *OrderedMap[T]) Set(key string, value T) {
	if !p.Has(key) {
		if len(p.keys) == 0 {
			p.keys = make([]string, 0, 10)
			p.values = make(map[string]T, 10)
		}

		p.keys = append(p.keys, key)
	}

	p.values[key] = value
}
