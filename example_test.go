// Copyright (c) 2026 the go-ruby-widgets/mvvm authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package mvvm_test

import (
	"fmt"

	"github.com/go-ruby-widgets/mvvm"
)

// Example mirrors the README: an Observable notifies through the drain queue, a
// Command queues an execute event for Ruby to run, and an ObservableList reports
// a granular collection change — every result a Ruby-shaped value.
func Example() {
	m := mvvm.NewModule()

	// A property: subscribe with a Ruby callback id, then change it.
	name := m.Observable("")
	name.Subscribe("on_name")
	name.Set("Ada")

	// A collection: observe it, then append.
	names := m.ObservableList(nil)
	names.Observe("on_names")
	names.Add("Ada")

	// An action: fire it once it is executable.
	save := m.Command("can_save", "do_save")
	save.SetCanExecute(true)
	save.Execute([]any{"now"})

	// Ruby drains the queue each tick and dispatches by "callback_id".
	for _, ev := range m.DrainEvents() {
		h := ev.(map[string]any)
		fmt.Println(h["callback_id"], h["kind"])
	}

	// Output:
	// on_name changed
	// on_names collection_changed
	// can_save can_execute_changed
	// do_save execute
}
