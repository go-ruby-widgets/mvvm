// Copyright (c) 2026 the go-ruby-widgets/mvvm authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package mvvm

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"

	wmvvm "github.com/go-widgets/mvvm"
)

// Module is the package-level Ruby receiver: the `Mvvm` module under rbgo. It is
// a factory for the three data-binding primitives — Observable, Command and
// ObservableList — and owns the event queue those primitives write to. Because a
// hosted Go library cannot call back into the Ruby runtime, every notification
// (an observable change, a command execution, a collection change) is appended
// to this queue as a Ruby-shaped Hash; the rbgo binding drains it with
// DrainEvents on each UI tick and dispatches each Hash to the Ruby callback
// named by its "callback_id". A Module is NOT safe for concurrent use — drive it
// from the UI goroutine, matching the single-threaded contract of the underlying
// github.com/go-widgets/mvvm primitives.
type Module struct {
	events []any // pending events, each a map[string]any (a Ruby Hash)
}

// NewModule returns a fresh Module with its own, independent event queue. The
// package-level convenience functions (Observable, Command, ObservableList and
// DrainEvents) delegate to a shared default Module instead.
func NewModule() *Module { return &Module{} }

var mod = NewModule()

// enqueue appends one Ruby-shaped event Hash to the module's drain queue.
func (m *Module) enqueue(ev map[string]any) { m.events = append(m.events, ev) }

// DrainEvents returns every event queued since the last drain (an Array of
// Hashes) and empties the queue. Each Hash carries a "callback_id" naming the
// Ruby callback to invoke plus a "kind" and its payload. This is the pull half
// of the callback seam: nothing here calls Ruby, so the rbgo binding polls this
// once per UI tick and dispatches. When nothing is pending it returns an empty
// Array.
func (m *Module) DrainEvents() []any {
	out := m.events
	m.events = nil
	if out == nil {
		return []any{}
	}
	return out
}

// Observable creates an Observable handle seeded with initial (any Ruby scalar,
// Hash or Array). Equality uses reflect.DeepEqual so that Hash- and Array-valued
// observables are compared by content rather than by an identity that would
// panic under ==; setting a deeply-equal value is a no-op that keeps two-way
// bindings loop-free.
func (m *Module) Observable(initial any) *Observable {
	o := &Observable{m: m, subs: map[int]func(){}}
	o.obs = wmvvm.NewObservableEq[any](initial, func(a, b any) bool {
		return reflect.DeepEqual(a, b)
	})
	return o
}

// Command creates a Command handle. canExecuteID and executeID are opaque Ruby
// callback identifiers (any value, or nil for none): executeID names the action
// run when the command fires, canExecuteID names the observer notified whenever
// executability changes. A new command is executable until SetCanExecute says
// otherwise.
func (m *Module) Command(canExecuteID, executeID any) *Command {
	c := &Command{m: m, canExecuteID: canExecuteID, executeID: executeID, can: true}
	c.cmd = wmvvm.NewCommand(
		func() {
			if executeID != nil {
				m.enqueue(map[string]any{
					"callback_id": executeID,
					"kind":        "execute",
					"args":        c.args,
				})
			}
		},
		func() bool { return c.can },
	)
	c.cmd.SubscribeCanExecuteChanged(func() {
		if canExecuteID != nil {
			m.enqueue(map[string]any{
				"callback_id": canExecuteID,
				"kind":        "can_execute_changed",
			})
		}
	})
	return c
}

// ObservableList creates an ObservableList handle seeded with items (a Ruby
// Array; nil for empty).
func (m *Module) ObservableList(items []any) *ObservableList {
	return &ObservableList{m: m, list: wmvvm.NewObservableList[any](items...), subs: map[int]func(){}}
}

// Observable is a Ruby-facing handle over a github.com/go-widgets/mvvm
// Observable[any] plus a table of drain-backed subscriptions.
type Observable struct {
	m      *Module
	obs    *wmvvm.Observable[any]
	subs   map[int]func() // subscription id -> unsubscribe
	nextID int
}

// Get returns the current value.
func (o *Observable) Get() any { return o.obs.Get() }

// Set assigns v and, when it differs (per reflect.DeepEqual) from the current
// value, queues a change event for every subscriber.
func (o *Observable) Set(v any) { o.obs.Set(v) }

// Subscribe registers the Ruby callback callbackID and returns an Int
// subscription id for Unsubscribe. On each change the observable queues a Hash
// {"callback_id"=>callbackID, "kind"=>"changed", "value"=>newValue} that
// DrainEvents later yields.
func (o *Observable) Subscribe(callbackID any) int {
	id := o.nextID
	o.nextID++
	unsub := o.obs.Subscribe(func(v any) {
		o.m.enqueue(map[string]any{
			"callback_id": callbackID,
			"kind":        "changed",
			"value":       v,
		})
	})
	o.subs[id] = unsub
	return id
}

// Unsubscribe detaches the subscription created by Subscribe. An unknown id is
// ignored.
func (o *Observable) Unsubscribe(subID int) {
	if unsub, ok := o.subs[subID]; ok {
		unsub()
		delete(o.subs, subID)
	}
}

// Command is a Ruby-facing handle over a github.com/go-widgets/mvvm Command. Its
// action and executability are owned by Ruby: firing the command queues an
// execute event for Ruby to run, and SetCanExecute records the executability
// Ruby has computed.
type Command struct {
	m            *Module
	cmd          *wmvvm.Command
	canExecuteID any
	executeID    any
	can          bool
	args         []any
}

// CanExecute reports whether the command may run right now.
func (c *Command) CanExecute() bool { return c.cmd.CanExecute() }

// Execute fires the command with args (a Ruby Array; nil for none). When
// CanExecute is true this queues an execute event carrying the args for Ruby to
// run; when false it is a no-op.
func (c *Command) Execute(args []any) {
	c.args = args
	c.cmd.Execute()
}

// SetCanExecute records the command's executability and notifies every
// can-execute observer (queueing a can-execute-changed event when the command
// was built with a canExecuteID).
func (c *Command) SetCanExecute(can bool) {
	c.can = can
	c.cmd.RaiseCanExecuteChanged()
}

// RaiseCanExecuteChanged notifies every can-execute observer without changing
// the executability — use it when the Ruby predicate's inputs changed.
func (c *Command) RaiseCanExecuteChanged() { c.cmd.RaiseCanExecuteChanged() }

// ObservableList is a Ruby-facing handle over a github.com/go-widgets/mvvm
// ObservableList[any] plus a table of drain-backed observers.
type ObservableList struct {
	m      *Module
	list   *wmvvm.ObservableList[any]
	subs   map[int]func() // observer id -> unsubscribe
	nextID int
}

// Add appends v and queues a collection-changed event.
func (l *ObservableList) Add(v any) { l.list.Append(v) }

// Insert places v at index i (clamped to [0, size]) and queues a
// collection-changed event.
func (l *ObservableList) Insert(i int, v any) { l.list.Insert(i, v) }

// RemoveAt removes the item at i and queues a collection-changed event. An
// out-of-range index is ignored.
func (l *ObservableList) RemoveAt(i int) { l.list.RemoveAt(i) }

// Set replaces the item at i and queues a collection-changed event. An
// out-of-range index is ignored.
func (l *ObservableList) Set(i int, v any) { l.list.Set(i, v) }

// Move relocates the item at from to to and queues a collection-changed event.
// An out-of-range index, or from == to, is ignored.
func (l *ObservableList) Move(from, to int) { l.list.Move(from, to) }

// Clear removes every item and queues a reset collection-changed event.
func (l *ObservableList) Clear() { l.list.Clear() }

// Get returns the item at i, or an error (a Ruby IndexError under rbgo) when i
// is out of range — the fetch-style accessor. It also demonstrates Call's
// trailing-error unwrapping.
func (l *ObservableList) Get(i int) (any, error) {
	if i < 0 || i >= l.list.Len() {
		return nil, fmt.Errorf("mvvm: index %d out of range (size %d)", i, l.list.Len())
	}
	return l.list.At(i), nil
}

// Size returns the item count.
func (l *ObservableList) Size() int { return l.list.Len() }

// Slice returns a defensive copy of the items as a Ruby Array.
func (l *ObservableList) Slice() []any { return l.list.Slice() }

// Observe registers the Ruby callback callbackID and returns an Int observer id
// for Unobserve. On each mutation the list queues a Ruby-shaped
// collection-changed Hash — {"callback_id"=>callbackID, "kind"=>
// "collection_changed", "action"=>..., "index"=>..., "to"=>..., "count"=>...,
// "items"=>...} — that DrainEvents later yields. "action" is one of "insert",
// "remove", "replace", "move" or "reset"; "to" is meaningful for "move" and
// "items" for "insert"/"replace".
func (l *ObservableList) Observe(callbackID any) int {
	id := l.nextID
	l.nextID++
	unsub := l.list.Subscribe(func(ev wmvvm.ListEvent[any]) {
		l.m.enqueue(map[string]any{
			"callback_id": callbackID,
			"kind":        "collection_changed",
			"action":      listAction(ev.Kind),
			"index":       ev.Index,
			"to":          ev.To,
			"count":       ev.Count,
			"items":       append([]any(nil), ev.Items...),
		})
	})
	l.subs[id] = unsub
	return id
}

// Unobserve detaches the observer created by Observe. An unknown id is ignored.
func (l *ObservableList) Unobserve(subID int) {
	if unsub, ok := l.subs[subID]; ok {
		unsub()
		delete(l.subs, subID)
	}
}

// listAction maps a go-widgets ListChangeKind to its Ruby action name.
func listAction(k wmvvm.ListChangeKind) string {
	switch k {
	case wmvvm.ListInsert:
		return "insert"
	case wmvvm.ListRemove:
		return "remove"
	case wmvvm.ListReplace:
		return "replace"
	case wmvvm.ListMove:
		return "move"
	default: // wmvvm.ListReset
		return "reset"
	}
}

// --- dynamic dispatch (the uniform surface rbgo binds via method_missing) ---

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Call dispatches a Ruby-style snake_case method name to the matching exported
// method of recv (a *Module, *Observable, *Command or *ObservableList), coercing
// each Ruby-supplied argument to the Go parameter type. A trailing "?" or "!"
// (Ruby predicate/bang convention) is ignored. Trailing arguments may be
// omitted; they default to nil. The result is the method's Ruby-shaped return
// value (or nil for a method that returns nothing); a trailing error return is
// unwrapped into Call's own error. This is the single entry point an rbgo
// binding drives.
func Call(recv any, method string, args ...any) (any, error) {
	rv := reflect.ValueOf(recv)
	if !rv.IsValid() {
		return nil, fmt.Errorf("mvvm: nil receiver")
	}
	m := rv.MethodByName(camelize(method))
	if !m.IsValid() {
		return nil, fmt.Errorf("mvvm: unknown method %q", method)
	}
	mt := m.Type()
	if len(args) > mt.NumIn() {
		return nil, fmt.Errorf("mvvm: %s takes at most %d argument(s), got %d",
			method, mt.NumIn(), len(args))
	}

	in := make([]reflect.Value, mt.NumIn())
	for i := 0; i < mt.NumIn(); i++ {
		var arg any
		if i < len(args) {
			arg = args[i]
		}
		v, err := coerce(arg, mt.In(i))
		if err != nil {
			return nil, fmt.Errorf("mvvm: %s argument %d: %w", method, i+1, err)
		}
		in[i] = v
	}

	outs := m.Call(in)
	if n := len(outs); n > 0 && mt.Out(n-1) == errorType {
		if e := outs[n-1]; !e.IsNil() {
			return nil, e.Interface().(error)
		}
		outs = outs[:n-1]
	}
	if len(outs) == 0 {
		return nil, nil
	}
	return outs[0].Interface(), nil
}

// Methods lists, sorted, the Ruby-style snake_case names Call accepts for recv.
func Methods(recv any) []string {
	t := reflect.TypeOf(recv)
	names := make([]string, 0, t.NumMethod())
	for i := 0; i < t.NumMethod(); i++ {
		names = append(names, snakeize(t.Method(i).Name))
	}
	sort.Strings(names)
	return names
}

// coerce converts a Ruby-supplied argument into the Go type target expects.
func coerce(val any, target reflect.Type) (reflect.Value, error) {
	if val != nil {
		if vv := reflect.ValueOf(val); vv.Type().AssignableTo(target) {
			return vv, nil
		}
	}
	switch target.Kind() {
	case reflect.Interface: // any — only reached when val is nil (a non-nil value
		// is always assignable to interface{} and returns via the fast path above)
		return reflect.Zero(target), nil
	case reflect.Int:
		n, ok := toInt(val)
		if !ok {
			return reflect.Value{}, fmt.Errorf("cannot use %T as an integer", val)
		}
		return reflect.ValueOf(n), nil
	case reflect.Bool:
		return reflect.ValueOf(truthy(val)), nil
	case reflect.Slice: // a Ruby Array ([]any); nil becomes a nil slice
		if val == nil {
			return reflect.Zero(target), nil
		}
		return reflect.Value{}, fmt.Errorf("cannot use %T as %s", val, target)
	}
	return reflect.Value{}, fmt.Errorf("mvvm: unsupported parameter type %s", target)
}

// toInt coerces a Ruby number to an int.
func toInt(val any) (int, bool) {
	switch x := val.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

// truthy applies Ruby truthiness: only false and nil are falsey.
func truthy(val any) bool {
	switch x := val.(type) {
	case nil:
		return false
	case bool:
		return x
	default:
		return true
	}
}

// camelize turns a snake_case (optionally ?/!-suffixed) method name into its
// exported Go method name.
func camelize(s string) string {
	s = strings.TrimRight(s, "?!")
	var b strings.Builder
	for _, w := range strings.Split(s, "_") {
		if w == "" {
			continue
		}
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

// snakeize is the inverse: an exported Go method name to snake_case.
func snakeize(s string) string {
	var b []rune
	rs := []rune(s)
	for i, r := range rs {
		if unicode.IsUpper(r) {
			prevLower := i > 0 && unicode.IsLower(rs[i-1])
			nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
			if i > 0 && (prevLower || nextLower) {
				b = append(b, '_')
			}
			r = unicode.ToLower(r)
		}
		b = append(b, r)
	}
	return string(b)
}

// --- package-level convenience over the default Module ---

// Default returns the shared default Module that the Ruby `Mvvm` receiver binds
// to. Its factory names (Observable, Command, ObservableList) match the handle
// type names, so the convenience surface is the module itself rather than
// same-named package functions: use Default().Observable(v), or drive it
// dynamically with Call(mvvm.Default(), "observable", v).
func Default() *Module { return mod }

// DrainEvents drains the default module's event queue (see Module.DrainEvents).
func DrainEvents() []any { return mod.DrainEvents() }
