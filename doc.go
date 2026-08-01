// Copyright (c) 2026 the go-ruby-widgets/mvvm authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package mvvm is the pure-Go, Ruby-runtime-independent core of the Ruby `mvvm`
// gem: the data-binding layer of the go-widgets ecosystem — an Observable
// property, a Command action and an ObservableList collection — shaped so that
// github.com/go-embedded-ruby/ruby (rbgo) can bind it as `require "mvvm"`.
//
// It is a thin adapter over the dependency-free primitives of
// github.com/go-widgets/mvvm. It exposes them through Ruby-facing handles
// (Module, Observable, Command, ObservableList) whose methods take and return
// Ruby-shaped values: a Hash (map[string]any), an Array ([]any) or a scalar. A
// single dynamic entry point, Call, dispatches a Ruby-style snake_case method
// name to the matching handle method and coerces the arguments, which is exactly
// what an rbgo binding drives from method_missing. Nothing here imports the Ruby
// runtime, so the package is equally usable as a standalone Go library — a
// sibling of go-ruby-regexp/regexp, go-ruby-erb/erb and go-ruby-opentype/opentype.
//
// # The callback seam
//
// A hosted Go library cannot synchronously call a Ruby block, so this package
// does not try to. Instead every notification — an observable change, a command
// execution, a collection change — is a registered callback id plus a queued,
// Ruby-shaped event Hash. The rbgo binding polls the queue with DrainEvents once
// per UI tick and dispatches each Hash to the Ruby callback named by its
// "callback_id". This "id + drain" pull model is the whole seam: Subscribe /
// Observe take a callback id and return a subscription id; DrainEvents yields the
// pending events; Ruby owns the actual blocks.
//
// # Handles
//
//   - Module is the package-level receiver (the `Mvvm` module under rbgo): it
//     builds Observables, Commands and ObservableLists and owns the event queue
//     they write to (DrainEvents).
//   - Observable is a bindable property: Get, Set, Subscribe(callback_id) and
//     Unsubscribe(sub_id). Values may be any Ruby scalar, Hash or Array;
//     equality is by content (reflect.DeepEqual), so setting an equal value is a
//     no-op.
//   - Command is a bindable action: CanExecute, Execute(args), SetCanExecute and
//     RaiseCanExecuteChanged. Firing it queues an execute event for Ruby to run.
//   - ObservableList is a bindable collection: Add, Insert, RemoveAt, Set, Move,
//     Clear, Get, Size, Slice, Observe(callback_id) and Unobserve(sub_id). Each
//     mutation queues a Ruby-shaped collection-changed event
//     {action:, index:, items:, ...}.
//
// # Usage from Go
//
//	m := mvvm.NewModule()
//	name := m.Observable("")
//	name.Subscribe("on_name")           // "on_name" is a Ruby callback id
//	name.Set("Ada")                     // queues {callback_id:"on_name", ...}
//	events := m.DrainEvents()           // an Array of Hashes for Ruby to dispatch
//
// # Usage from Ruby
//
// Under rbgo, `require "mvvm"` gives an `Mvvm` module whose snake_case methods
// are these operations, returning Ruby Hashes, Arrays and scalars:
//
//	require "mvvm"
//
//	name = Mvvm.observable("")
//	name.subscribe(:on_name)
//	name.set("Ada")
//
//	names = Mvvm.observable_list([])
//	names.observe(:on_names)
//	save = Mvvm.command(:can_save, :do_save)
//	save.set_can_execute(true)
//	save.execute([])                    # queues {callback_id: :do_save, ...}
//
//	Mvvm.drain_events.each do |ev|      # => Array<Hash>
//	  dispatch(ev[:callback_id], ev)    # Ruby runs the actual block
//	end
//
// The `require "mvvm"` binding lives in rbgo (a thin method_missing shim over
// Call that maintains the callback-id table and drains); it is pending in that
// repo.
package mvvm
