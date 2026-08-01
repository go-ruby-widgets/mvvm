// Copyright (c) 2026 the go-ruby-widgets/mvvm authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package mvvm

import (
	"reflect"
	"testing"
)

// intType and boolType are handy reflect.Type targets for coerce.
var (
	intType   = reflect.TypeOf(int(0))
	boolType  = reflect.TypeOf(false)
	floatType = reflect.TypeOf(float64(0))
	anyType   = reflect.TypeOf((*any)(nil)).Elem()
	sliceType = reflect.TypeOf([]any(nil))
)

func TestCoerceNumbers(t *testing.T) {
	// int, int64 and float64 all coerce to a Go int.
	for _, in := range []any{int(5), int64(5), float64(5)} {
		v, err := coerce(in, intType)
		if err != nil || v.Interface().(int) != 5 {
			t.Fatalf("coerce(%T) = %v, %v", in, v, err)
		}
	}
	// A non-number is rejected.
	if _, err := coerce("x", intType); err == nil {
		t.Fatal("coerce(string -> int) should error")
	}
}

func TestCoerceInterfaceAndSlice(t *testing.T) {
	// A nil into an interface (any) target yields a valid zero Value.
	v, err := coerce(nil, anyType)
	if err != nil || v.Interface() != nil {
		t.Fatalf("coerce(nil -> any) = %v, %v", v, err)
	}
	// A nil into a slice target yields a nil slice.
	v, err = coerce(nil, sliceType)
	if err != nil || !v.IsNil() {
		t.Fatalf("coerce(nil -> []any) = %v, %v", v, err)
	}
	// A non-nil assignable value takes the fast path.
	v, err = coerce([]any{1}, sliceType)
	if err != nil || v.Len() != 1 {
		t.Fatalf("coerce([]any -> []any) = %v, %v", v, err)
	}
	// A non-assignable value into a slice target errors.
	if _, err := coerce(7, sliceType); err == nil {
		t.Fatal("coerce(int -> []any) should error")
	}
}

func TestCoerceBoolAndUnsupported(t *testing.T) {
	// truthiness through coerce.
	for in, want := range map[any]bool{true: true, false: false, nil: false, "s": true} {
		v, err := coerce(in, boolType)
		if err != nil || v.Interface().(bool) != want {
			t.Fatalf("coerce(%v -> bool) = %v, %v; want %v", in, v, err, want)
		}
	}
	// An unsupported parameter kind (float64) with a non-assignable value falls
	// through to the "unsupported parameter type" error.
	if _, err := coerce("x", floatType); err == nil {
		t.Fatal("coerce(string -> float64) should error (unsupported)")
	}
}

func TestToIntAndTruthy(t *testing.T) {
	for _, c := range []struct {
		in any
		n  int
		ok bool
	}{
		{int(3), 3, true},
		{int64(3), 3, true},
		{float64(3), 3, true},
		{"x", 0, false},
	} {
		if n, ok := toInt(c.in); n != c.n || ok != c.ok {
			t.Fatalf("toInt(%T) = %d,%v", c.in, n, ok)
		}
	}
	for in, want := range map[any]bool{nil: false, true: true, false: false, 0: true, "": true} {
		if got := truthy(in); got != want {
			t.Fatalf("truthy(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestCamelizeSnakeize(t *testing.T) {
	for in, want := range map[string]string{
		"observable_list": "ObservableList",
		"can_execute?":    "CanExecute", // trailing predicate marker dropped
		"do_it!":          "DoIt",       // trailing bang dropped
		"_leading__gap_":  "LeadingGap", // empty segments skipped
		"get":             "Get",
	} {
		if got := camelize(in); got != want {
			t.Fatalf("camelize(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"CanExecute":     "can_execute",
		"RemoveAt":       "remove_at",
		"ID":             "id", // consecutive capitals stay together
		"ParseURL":       "parse_url",
		"observableList": "observable_list",
	} {
		if got := snakeize(in); got != want {
			t.Fatalf("snakeize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestListActionAll(t *testing.T) {
	// Directly exercise every mapping, including the reset default.
	got := []string{
		listAction(0), // ListInsert
		listAction(1), // ListRemove
		listAction(2), // ListReplace
		listAction(3), // ListMove
		listAction(4), // ListReset (default)
	}
	want := []string{"insert", "remove", "replace", "move", "reset"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listAction = %v, want %v", got, want)
	}
}
