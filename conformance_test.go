package sfv

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// testCase - httpwg/structured-field-tests の JSON 1 エントリ。
type testCase struct {
	Name       string          `json:"name"`
	Raw        []string        `json:"raw"`
	HeaderType string          `json:"header_type"`
	Expected   json.RawMessage `json:"expected"`
	MustFail   bool            `json:"must_fail"`
	CanFail    bool            `json:"can_fail"`
	Canonical  []string        `json:"canonical"`
}

var phase1Files = []string{
	"number.json",
	"number-generated.json",
	"string.json",
	"string-generated.json",
	"token.json",
	"token-generated.json",
	"binary.json",
	"boolean.json",
}

var phase2Files = []string{
	"date.json",
	"display-string.json",
	"param-dict.json",
	"param-list.json",
	"param-listlist.json",
	"key-generated.json",
}

var phase3Files = []string{
	"list.json",
	"dictionary.json",
	"listlist.json",
	"examples.json",
	"large-generated.json",
}

const testDir = "tests/structured-field-tests"

func TestConformance(t *testing.T) {
	files := make([]string, 0, len(phase1Files)+len(phase2Files)+len(phase3Files))
	files = append(files, phase1Files...)
	files = append(files, phase2Files...)
	files = append(files, phase3Files...)

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			runTestFile(t, filepath.Join(testDir, name))
		})
	}
}

func runTestFile(t *testing.T, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var cases []testCase
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&cases); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func runTestCase(t *testing.T, tc testCase) {
	t.Helper()

	parsed, parseErr := parseByHeaderType(tc.HeaderType, tc.Raw)

	if tc.MustFail {
		if parseErr == nil {
			t.Errorf("must_fail=true だが成功した: raw=%q parsed=%#v", tc.Raw, parsed)
		}
		return
	}

	if parseErr != nil {
		if tc.CanFail {
			t.Skipf("can_fail=true: %v", parseErr)
		}
		t.Fatalf("想定外のパース失敗: raw=%q err=%v", tc.Raw, parseErr)
	}

	expected, err := decodeExpected(tc.HeaderType, tc.Expected)
	if err != nil {
		t.Fatalf("期待値デコード失敗: %v (raw=%s)", err, string(tc.Expected))
	}

	if !valueEqual(parsed, expected) {
		t.Errorf("結果不一致\n  got:      %s\n  expected: %s\n  raw:      %q",
			formatValue(parsed), formatValue(expected), tc.Raw)
	}
}

func parseByHeaderType(headerType string, raw []string) (any, error) {
	switch headerType {
	case "item":
		return ParseItem(raw)
	case "list":
		return ParseList(raw)
	case "dictionary":
		return ParseDictionary(raw)
	}
	return nil, fmt.Errorf("unknown header_type: %s", headerType)
}

func decodeExpected(headerType string, raw json.RawMessage) (any, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}

	switch headerType {
	case "item":
		return decodeExpectedItem(v)
	case "list":
		return decodeExpectedList(v)
	case "dictionary":
		return decodeExpectedDictionary(v)
	}
	return nil, fmt.Errorf("unknown header_type: %s", headerType)
}

func decodeExpectedItem(v any) (*Item, error) {
	arr, ok := v.([]any)
	if !ok || len(arr) != 2 {
		return nil, fmt.Errorf("item は2要素配列であるべき: %T %v", v, v)
	}
	bi, err := decodeExpectedBareItem(arr[0])
	if err != nil {
		return nil, err
	}
	params, err := decodeExpectedParameters(arr[1])
	if err != nil {
		return nil, err
	}
	return &Item{Value: bi, Parameters: params}, nil
}

func decodeExpectedInnerList(v any) (*InnerList, error) {
	arr, ok := v.([]any)
	if !ok || len(arr) != 2 {
		return nil, fmt.Errorf("inner list は2要素配列であるべき: %T %v", v, v)
	}
	inner, ok := arr[0].([]any)
	if !ok {
		return nil, fmt.Errorf("inner list の1要素目は配列であるべき: %T", arr[0])
	}
	var items []*Item
	for _, e := range inner {
		item, err := decodeExpectedItem(e)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	params, err := decodeExpectedParameters(arr[1])
	if err != nil {
		return nil, err
	}
	return &InnerList{Items: items, Parameters: params}, nil
}

func decodeExpectedList(v any) (List, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("list は配列であるべき: %T", v)
	}
	result := make(List, 0, len(arr))
	for _, e := range arr {
		m, err := decodeExpectedListMember(e)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, nil
}

// decodeExpectedListMember - inner list は JSON で [[..], [..]] 形なので、
// 1要素目が配列かどうかで Item と判別する。
func decodeExpectedListMember(v any) (ListMember, error) {
	arr, ok := v.([]any)
	if !ok || len(arr) != 2 {
		return nil, fmt.Errorf("member は2要素配列: %T %v", v, v)
	}
	if _, isArr := arr[0].([]any); isArr {
		return decodeExpectedInnerList(v)
	}
	return decodeExpectedItem(v)
}

func decodeExpectedDictionary(v any) (*Dictionary, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("dictionary は配列であるべき: %T", v)
	}
	result := new(Dictionary)
	for _, e := range arr {
		entry, ok := e.([]any)
		if !ok || len(entry) != 2 {
			return nil, fmt.Errorf("dictionary entry は2要素配列: %v", e)
		}
		key, ok := entry[0].(string)
		if !ok {
			return nil, fmt.Errorf("dictionary key は string: %T", entry[0])
		}
		memberValue, err := decodeExpectedDictionaryMember(entry[1])
		if err != nil {
			return nil, err
		}
		result.Set(key, memberValue)
	}
	return result, nil
}

func decodeExpectedDictionaryMember(v any) (DictionaryMemberValue, error) {
	arr, ok := v.([]any)
	if !ok || len(arr) != 2 {
		return nil, fmt.Errorf("dict member は2要素配列: %v", v)
	}
	if _, isArr := arr[0].([]any); isArr {
		il, err := decodeExpectedInnerList(v)
		if err != nil {
			return nil, err
		}
		return il, nil
	}
	item, err := decodeExpectedItem(v)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func decodeExpectedParameters(v any) (Parameters, error) {
	var params Parameters
	arr, ok := v.([]any)
	if !ok {
		return params, fmt.Errorf("parameters は配列であるべき: %T", v)
	}
	for _, e := range arr {
		entry, ok := e.([]any)
		if !ok || len(entry) != 2 {
			return params, fmt.Errorf("param entry は2要素配列: %v", e)
		}
		key, ok := entry[0].(string)
		if !ok {
			return params, fmt.Errorf("param key は string: %T", entry[0])
		}
		val, err := decodeExpectedBareItem(entry[1])
		if err != nil {
			return params, err
		}
		params.Set(key, val)
	}
	return params, nil
}

func decodeExpectedBareItem(v any) (BareItem, error) {
	switch x := v.(type) {
	case json.Number:
		if strings.ContainsAny(string(x), ".eE") {
			f, err := x.Float64()
			if err != nil {
				return nil, err
			}
			return Decimal(f), nil
		}
		i, err := x.Int64()
		if err != nil {
			return nil, err
		}
		return Integer(i), nil

	case string:
		return String(x), nil

	case bool:
		return Boolean(x), nil

	case map[string]any:
		tag, _ := x["__type"].(string)
		value := x["value"]

		switch tag {
		case "token":
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("token value は string: %T", value)
			}
			return Token(s), nil

		case "binary":
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("binary value は string: %T", value)
			}
			// RFC 4648 §6 base32。padding が省略される実装にも対応するため両方を試す。
			dec, err := base32.StdEncoding.DecodeString(s)
			if err != nil {
				dec, err = base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
				if err != nil {
					return nil, fmt.Errorf("base32 decode %q: %w", s, err)
				}
			}
			return ByteSequence(dec), nil

		case "date":
			n, ok := value.(json.Number)
			if !ok {
				return nil, fmt.Errorf("date value は number: %T", value)
			}
			i, err := n.Int64()
			if err != nil {
				return nil, err
			}
			return Date(i), nil

		case "displaystring":
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("displaystring value は string: %T", value)
			}
			return DisplayString(s), nil
		}
		return nil, fmt.Errorf("unknown __type: %q", tag)
	}

	return nil, fmt.Errorf("unknown bare item json type: %T %v", v, v)
}

// valueEqual - パース結果と期待値の構造的等価性を比較する。Parameters は OrderedMap なので
// 順序込みで比較する必要がある。
func valueEqual(got, expected any) bool {
	switch g := got.(type) {
	case *Item:
		e, ok := expected.(*Item)
		return ok && itemEqual(g, e)
	case List:
		e, ok := expected.(List)
		return ok && listEqual(g, e)
	case *Dictionary:
		e, ok := expected.(*Dictionary)
		return ok && dictionaryEqual(g, e)
	}
	return false
}

func itemEqual(a, b *Item) bool {
	if a == nil || b == nil {
		return a == b
	}
	if !bareItemEqual(a.Value, b.Value) {
		return false
	}
	return parametersEqual(a.Parameters, b.Parameters)
}

func innerListEqual(a, b *InnerList) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.Items) != len(b.Items) {
		return false
	}
	for i := range a.Items {
		if !itemEqual(a.Items[i], b.Items[i]) {
			return false
		}
	}
	return parametersEqual(a.Parameters, b.Parameters)
}

func listEqual(a, b List) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !listMemberEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// listMemberEqual - ListMember の等価性。値型/ポインタ型の両方に対応する。
func listMemberEqual(a, b ListMember) bool {
	switch av := a.(type) {
	case *Item:
		bv, ok := b.(*Item)
		return ok && itemEqual(av, bv)
	case *InnerList:
		bv, ok := b.(*InnerList)
		return ok && innerListEqual(av, bv)
	case Item:
		bv, ok := b.(Item)
		return ok && itemEqual(&av, &bv)
	case InnerList:
		bv, ok := b.(InnerList)
		return ok && innerListEqual(&av, &bv)
	}
	return false
}

func dictionaryEqual(a, b *Dictionary) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.keys) != len(b.keys) {
		return false
	}
	for i, k := range a.keys {
		if b.keys[i] != k {
			return false
		}
		if !dictionaryMemberEqual(a.values[k], b.values[k]) {
			return false
		}
	}
	return true
}

// dictionaryMemberEqual - Dictionary member の等価性。値型/ポインタ型の両方に対応する。
func dictionaryMemberEqual(a, b DictionaryMemberValue) bool {
	switch av := a.(type) {
	case Item:
		bv, ok := b.(Item)
		return ok && itemEqual(&av, &bv)
	case *Item:
		bv, ok := b.(*Item)
		return ok && itemEqual(av, bv)
	case InnerList:
		bv, ok := b.(InnerList)
		return ok && innerListEqual(&av, &bv)
	case *InnerList:
		bv, ok := b.(*InnerList)
		return ok && innerListEqual(av, bv)
	}
	return false
}

func parametersEqual(a, b Parameters) bool {
	if len(a.keys) != len(b.keys) {
		return false
	}
	for i, k := range a.keys {
		if b.keys[i] != k {
			return false
		}
		if !bareItemEqual(a.values[k], b.values[k]) {
			return false
		}
	}
	return true
}

func bareItemEqual(a, b BareItem) bool {
	return reflect.DeepEqual(a, b)
}

func formatValue(v any) string {
	switch x := v.(type) {
	case *Item:
		if x == nil {
			return "<nil Item>"
		}
		return fmt.Sprintf("Item{Value: %T(%v), Params: %s}", x.Value, x.Value, formatParameters(x.Parameters))
	case List:
		parts := make([]string, 0, len(x))
		for _, m := range x {
			parts = append(parts, formatValue(m))
		}
		return "List[" + strings.Join(parts, ", ") + "]"
	case *Dictionary:
		if x == nil {
			return "<nil Dictionary>"
		}
		parts := make([]string, 0, len(x.keys))
		for _, k := range x.keys {
			parts = append(parts, fmt.Sprintf("%s=%s", k, formatValue(x.values[k])))
		}
		return "Dictionary{" + strings.Join(parts, ", ") + "}"
	case *InnerList:
		if x == nil {
			return "<nil InnerList>"
		}
		parts := make([]string, 0, len(x.Items))
		for _, it := range x.Items {
			parts = append(parts, formatValue(it))
		}
		return "InnerList(" + strings.Join(parts, " ") + "); Params: " + formatParameters(x.Parameters)
	case Item:
		return formatValue(&x)
	case InnerList:
		return formatValue(&x)
	}
	return fmt.Sprintf("%#v", v)
}

func formatParameters(p Parameters) string {
	if len(p.keys) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(p.keys))
	for _, k := range p.keys {
		parts = append(parts, fmt.Sprintf("%s=%T(%v)", k, p.values[k], p.values[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
