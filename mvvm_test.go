// Copyright (c) 2026 the go-ruby-widgets/mvvm authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package mvvm_test

import (
	"reflect"
	"testing"

	"github.com/go-ruby-widgets/mvvm"
)

// drainKinds returns the (callback_id, kind) pairs of a drained queue, in order.
func drainKinds(evs []any) [][2]any {
	out := make([][2]any, len(evs))
	for i, e := range evs {
		h := e.(map[string]any)
		out[i] = [2]any{h["callback_id"], h["kind"]}
	}
	return out
}

func TestObservableGetSetNotify(t *testing.T) {
	m := mvvm.NewModule()
	o := m.Observable("start")
	if o.Get() != "start" {
		t.Fatalf("Get = %v, want start", o.Get())
	}

	sub := o.Subscribe("cb")

	// Setting an equal value is a no-op: no event queued.
	o.Set("start")
	if evs := m.DrainEvents(); len(evs) != 0 {
		t.Fatalf("equal Set queued %d events, want 0", len(evs))
	}

	// A real change queues one changed event carrying the new value.
	o.Set("next")
	if o.Get() != "next" {
		t.Fatalf("Get after Set = %v, want next", o.Get())
	}
	evs := m.DrainEvents()
	if len(evs) != 1 {
		t.Fatalf("change queued %d events, want 1", len(evs))
	}
	h := evs[0].(map[string]any)
	if h["callback_id"] != "cb" || h["kind"] != "changed" || h["value"] != "next" {
		t.Fatalf("event = %v", h)
	}

	// After Unsubscribe, changes stop queuing.
	o.Unsubscribe(sub)
	o.Unsubscribe(sub) // unknown id: no-op branch
	o.Set("third")
	if evs := m.DrainEvents(); len(evs) != 0 {
		t.Fatalf("after Unsubscribe queued %d events, want 0", len(evs))
	}
}

func TestObservableHashAndArrayValues(t *testing.T) {
	m := mvvm.NewModule()
	o := m.Observable(map[string]any{"a": 1})
	o.Subscribe("cb")

	// DeepEqual: an equal Hash is a no-op...
	o.Set(map[string]any{"a": 1})
	if evs := m.DrainEvents(); len(evs) != 0 {
		t.Fatalf("equal Hash Set queued %d, want 0", len(evs))
	}
	// ...a different Hash notifies.
	o.Set(map[string]any{"a": 2})
	if evs := m.DrainEvents(); len(evs) != 1 {
		t.Fatalf("changed Hash queued %d, want 1", len(evs))
	}

	// Array values are compared by content too (== would panic on slices).
	a := m.Observable([]any{1, 2})
	a.Set([]any{1, 2})    // equal -> ignored (would panic under ==)
	a.Set([]any{1, 2, 3}) // different
	if got := a.Get().([]any); len(got) != 3 {
		t.Fatalf("array Get = %v", got)
	}
}

func TestCommandCanAndExecute(t *testing.T) {
	m := mvvm.NewModule()
	c := m.Command("can_cb", "exec_cb")

	// New commands are executable.
	if !c.CanExecute() {
		t.Fatal("new command should be executable")
	}

	// Disable it: SetCanExecute records the state and fires a change event.
	c.SetCanExecute(false)
	if c.CanExecute() {
		t.Fatal("command should be disabled")
	}
	// Executing a disabled command is a no-op (no execute event).
	c.Execute([]any{"x"})
	if got := drainKinds(m.DrainEvents()); len(got) != 1 || got[0] != [2]any{"can_cb", "can_execute_changed"} {
		t.Fatalf("disabled flow events = %v", got)
	}

	// Enable and fire: an execute event carries the args.
	c.SetCanExecute(true)
	c.Execute([]any{"go", 1})
	evs := m.DrainEvents()
	// events: can_execute_changed (from SetCanExecute), then execute.
	kinds := drainKinds(evs)
	if len(kinds) != 2 || kinds[1] != [2]any{"exec_cb", "execute"} {
		t.Fatalf("enabled flow events = %v", kinds)
	}
	args := evs[1].(map[string]any)["args"].([]any)
	if !reflect.DeepEqual(args, []any{"go", 1}) {
		t.Fatalf("execute args = %v", args)
	}

	// RaiseCanExecuteChanged without a state change still notifies.
	c.RaiseCanExecuteChanged()
	if got := drainKinds(m.DrainEvents()); len(got) != 1 || got[0] != [2]any{"can_cb", "can_execute_changed"} {
		t.Fatalf("raise-only events = %v", got)
	}
}

func TestCommandNilCallbackIDs(t *testing.T) {
	m := mvvm.NewModule()
	// nil ids: neither the execute nor the can-execute-changed event is queued.
	c := m.Command(nil, nil)
	c.SetCanExecute(true) // raise with nil can id -> no event
	c.Execute(nil)        // execute with nil exec id -> no event
	if evs := m.DrainEvents(); len(evs) != 0 {
		t.Fatalf("nil-id command queued %d events, want 0", len(evs))
	}
}

func TestObservableListMutationsAndEvents(t *testing.T) {
	m := mvvm.NewModule()
	l := m.ObservableList([]any{"a", "b"})
	if l.Size() != 2 {
		t.Fatalf("Size = %d, want 2", l.Size())
	}
	obs := l.Observe("lcb")

	// Exercise every mutation; collect the action names it reports.
	l.Add("c")       // insert
	l.Insert(0, "z") // insert
	l.Set(1, "A")    // replace
	l.Move(0, 2)     // move
	l.RemoveAt(0)    // remove
	l.RemoveAt(999)  // out of range: ignored, no event
	l.Clear()        // reset

	evs := m.DrainEvents()
	var actions []string
	for _, e := range evs {
		h := e.(map[string]any)
		if h["kind"] != "collection_changed" || h["callback_id"] != "lcb" {
			t.Fatalf("unexpected list event %v", h)
		}
		actions = append(actions, h["action"].(string))
	}
	want := []string{"insert", "insert", "replace", "move", "remove", "reset"}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("actions = %v, want %v", actions, want)
	}

	// After Unobserve, mutations stop queuing.
	l.Unobserve(obs)
	l.Unobserve(obs) // unknown id: no-op branch
	l.Add("late")
	if evs := m.DrainEvents(); len(evs) != 0 {
		t.Fatalf("after Unobserve queued %d, want 0", len(evs))
	}

	// Slice is a defensive copy; Get is bounds-checked.
	if sl := l.Slice(); len(sl) != 1 || sl[0] != "late" {
		t.Fatalf("Slice = %v", sl)
	}
	if v, err := l.Get(0); err != nil || v != "late" {
		t.Fatalf("Get(0) = %v, %v", v, err)
	}
	if _, err := l.Get(5); err == nil {
		t.Fatal("Get(5) should error (out of range)")
	}
	if _, err := l.Get(-1); err == nil {
		t.Fatal("Get(-1) should error (out of range)")
	}
}

func TestDrainEventsEmpty(t *testing.T) {
	m := mvvm.NewModule()
	if evs := m.DrainEvents(); evs == nil || len(evs) != 0 {
		t.Fatalf("empty drain = %v, want non-nil empty slice", evs)
	}
}

func TestDefaultModuleAndPackageDrain(t *testing.T) {
	o := mvvm.Default().Observable(0)
	o.Subscribe("d")
	o.Set(1)
	if evs := mvvm.DrainEvents(); len(evs) != 1 {
		t.Fatalf("package DrainEvents = %d, want 1", len(evs))
	}
}

// --- dynamic dispatch (Call / Methods) ---

func TestCallDispatch(t *testing.T) {
	m := mvvm.NewModule()

	// Build an observable via Call, then set/get through it.
	ov, err := mvvm.Call(m, "observable", "hi")
	if err != nil {
		t.Fatalf("observable: %v", err)
	}
	o := ov.(*mvvm.Observable)
	if _, err := mvvm.Call(o, "subscribe", "cb"); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	// A method returning nothing yields a nil result.
	if res, err := mvvm.Call(o, "set", "bye"); err != nil || res != nil {
		t.Fatalf("set: %v, %v", res, err)
	}
	got, err := mvvm.Call(o, "get")
	if err != nil || got != "bye" {
		t.Fatalf("get: %v, %v", got, err)
	}

	// A list via Call with an Array argument; a bang/predicate suffix is ignored.
	lv, _ := mvvm.Call(m, "observable_list", []any{1, 2, 3})
	l := lv.(*mvvm.ObservableList)
	if n, err := mvvm.Call(l, "size"); err != nil || n != 3 {
		t.Fatalf("size: %v, %v", n, err)
	}
	// Trailing-error unwrap: get(0) returns the value, error nil-trimmed.
	if v, err := mvvm.Call(l, "get", 0); err != nil || v != 1 {
		t.Fatalf("get(0): %v, %v", v, err)
	}
	// Trailing-error unwrap: get(9) surfaces the error.
	if _, err := mvvm.Call(l, "get", 9); err == nil {
		t.Fatal("get(9) via Call should error")
	}

	// A command via Call; predicate name with a trailing '?'.
	cv, _ := mvvm.Call(m, "command", nil, "e")
	c := cv.(*mvvm.Command)
	if ok, err := mvvm.Call(c, "can_execute?"); err != nil || ok != true {
		t.Fatalf("can_execute?: %v, %v", ok, err)
	}
	// execute with no argument: the []any parameter coerces from nil.
	if _, err := mvvm.Call(c, "execute"); err != nil {
		t.Fatalf("execute (no args): %v", err)
	}
	// set_can_execute with a bool coerced argument.
	if _, err := mvvm.Call(c, "set_can_execute", false); err != nil {
		t.Fatalf("set_can_execute: %v", err)
	}
}

func TestCallErrors(t *testing.T) {
	m := mvvm.NewModule()

	if _, err := mvvm.Call(nil, "observable"); err == nil {
		t.Fatal("nil receiver should error")
	}
	if _, err := mvvm.Call(m, "no_such_method"); err == nil {
		t.Fatal("unknown method should error")
	}
	// Too many arguments.
	if _, err := mvvm.Call(m, "observable", 1, 2); err == nil {
		t.Fatal("too many args should error")
	}
	l := m.ObservableList(nil)
	// Bad int coercion: a string where an index int is expected.
	if _, err := mvvm.Call(l, "insert", "notint", "v"); err == nil {
		t.Fatal("non-int index should error")
	}
	// Bad slice coercion: a scalar where an Array is expected.
	if _, err := mvvm.Call(m, "observable_list", 42); err == nil {
		t.Fatal("non-array where Array expected should error")
	}
}

func TestMethods(t *testing.T) {
	m := mvvm.NewModule()
	names := mvvm.Methods(m)
	want := map[string]bool{"observable": true, "command": true, "observable_list": true, "drain_events": true}
	for _, n := range names {
		delete(want, n)
	}
	if len(want) != 0 {
		t.Fatalf("Methods(Module) missing %v (got %v)", want, mvvm.Methods(m))
	}
	// A handle's snake_case method list is sorted and includes its accessors.
	ln := mvvm.Methods(m.ObservableList(nil))
	if !reflect.DeepEqual(ln, sortedStrings(ln)) {
		t.Fatalf("Methods not sorted: %v", ln)
	}
	if !contains(ln, "remove_at") || !contains(ln, "size") {
		t.Fatalf("ObservableList methods = %v", ln)
	}
}

func sortedStrings(s []string) []string {
	out := append([]string(nil), s...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
