# go-ruby-widgets/mvvm

[![CI](https://github.com/go-ruby-widgets/mvvm/actions/workflows/ci.yml/badge.svg)](https://github.com/go-ruby-widgets/mvvm/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-ruby-widgets/mvvm.svg)](https://pkg.go.dev/github.com/go-ruby-widgets/mvvm)

The pure-Go, Ruby-runtime-independent core of the Ruby **`mvvm`** gem — the
data-binding layer of the [go-widgets](https://github.com/go-widgets) ecosystem
(an **Observable** property, a **Command** action and an **ObservableList**
collection) — shaped so that
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) (`rbgo`) can bind it
as `require "mvvm"`.

It is a thin adapter over the dependency-free primitives of
[`go-widgets/mvvm`](https://github.com/go-widgets/mvvm):

| Primitive | Role |
| --- | --- |
| `Observable` | a bindable **property** — `get` / `set` / `subscribe`, content-equality no-op on equal values |
| `Command` | a bindable **action** — `can_execute?` / `execute`, with a can-execute-changed signal |
| `ObservableList` | a bindable **collection** — emitting granular insert / remove / replace / move / reset events |

It exposes them through Ruby-facing handles — `Module`, `Observable`, `Command`,
`ObservableList` — whose methods take and return **Ruby-shaped values**: a
**Hash** (`map[string]any`), an **Array** (`[]any`) or a scalar. A single dynamic
entry point, `Call`, dispatches a Ruby-style snake_case method name to the
matching handle method and coerces the arguments, which is exactly what an rbgo
binding drives from `method_missing`. Nothing here depends on the Ruby runtime,
so it is equally usable as a standalone Go library — a sibling of
`go-ruby-regexp/regexp`, `go-ruby-erb/erb` and `go-ruby-opentype/opentype`.

- **CGO-free**, builds and tests identically on `amd64`, `arm64`, `riscv64`,
  `loong64`, `ppc64le`, `s390x`, plus `js/wasm`.
- **100 % statement coverage**, race-clean, enforced in CI.

## The callback seam (id + drain)

A hosted Go library cannot synchronously call a Ruby block, so this package does
not try to. Instead every notification — an observable change, a command
execution, a collection change — is a **registered callback id** plus a
**queued, Ruby-shaped event Hash**. The rbgo binding polls the queue with
`drain_events` once per UI tick and dispatches each Hash to the Ruby callback
named by its `"callback_id"`:

```
Ruby block  ──registers──▶  callback id  ──stored on subscribe/observe/command
   ▲                                              │
   │                                     Go mutation queues an event Hash
   └────────dispatch──── drain_events ◀───────────┘   (once per UI tick)
```

`subscribe` / `observe` take a callback id and return an integer subscription id
(pass it to `unsubscribe` / `unobserve`); `drain_events` yields the pending
events and empties the queue. Ruby owns the actual blocks.

## The Ruby-facing surface

**`Module`** — the package-level receiver (the `Mvvm` module under rbgo):

| Method | Returns |
| --- | --- |
| `observable(initial)` | an `Observable` handle |
| `command(can_execute_id, execute_id)` | a `Command` handle |
| `observable_list(items)` | an `ObservableList` handle |
| `drain_events` | Array of event Hashes (drains the queue) |

**`Observable`** — a bindable property:

| Method | Returns |
| --- | --- |
| `get` | the current value (scalar, Hash or Array) |
| `set(value)` | — (queues a `changed` event when the value differs by content) |
| `subscribe(callback_id)` | an Int subscription id |
| `unsubscribe(sub_id)` | — |

**`Command`** — a bindable action:

| Method | Returns |
| --- | --- |
| `can_execute?` | Bool |
| `execute(args)` | — (queues an `execute` event carrying `args` when executable) |
| `set_can_execute(bool)` | — (records executability, fires a change) |
| `raise_can_execute_changed` | — |

**`ObservableList`** — a bindable collection:

| Method | Returns |
| --- | --- |
| `add(v)` / `insert(i, v)` / `set(i, v)` / `remove_at(i)` / `move(from, to)` / `clear` | — (each queues a collection-changed event) |
| `get(i)` | the item, or raises `IndexError` out of range |
| `size` | Int |
| `slice` | a defensive-copy Array |
| `observe(callback_id)` | an Int observer id |
| `unobserve(sub_id)` | — |

Each drained event is a Hash with a `"callback_id"`, a `"kind"` and its payload:

```
# observable change
{ "callback_id"=>, "kind"=>"changed", "value"=> }
# command
{ "callback_id"=>, "kind"=>"execute", "args"=>[...] }
{ "callback_id"=>, "kind"=>"can_execute_changed" }
# collection change (kind => "collection_changed")
{ "callback_id"=>, "action"=>"insert"|"remove"|"replace"|"move"|"reset",
  "index"=>, "to"=>, "count"=>, "items"=>[...] }
```

## Usage from Ruby

Under rbgo, `require "mvvm"` gives an `Mvvm` module whose snake_case methods are
these operations, returning Ruby Hashes, Arrays and scalars:

```ruby
require "mvvm"

name  = Mvvm.observable("")
name.subscribe(:on_name)

names = Mvvm.observable_list([])
names.observe(:on_names)

save  = Mvvm.command(:can_save, :do_save)
save.set_can_execute(true)

name.set("Ada")
names.add(name.get)
save.execute([])                       # queues {callback_id: :do_save, ...}

Mvvm.drain_events.each do |ev|         # => Array<Hash>, once per UI tick
  dispatch(ev[:callback_id], ev)       # Ruby runs the actual block
end
```

The `require "mvvm"` binding lives in rbgo (a thin `method_missing` shim over
`Call` that keeps the callback-id table and drains); it is pending in that repo.

## Install (Go)

```sh
go get github.com/go-ruby-widgets/mvvm
```

## Usage from Go

```go
package main

import (
	"fmt"

	"github.com/go-ruby-widgets/mvvm"
)

func main() {
	m := mvvm.NewModule()

	name := m.Observable("")
	name.Subscribe("on_name") // "on_name" is a Ruby callback id
	name.Set("Ada")           // queues a changed event

	// Ruby drains the queue each tick and dispatches by "callback_id".
	for _, ev := range m.DrainEvents() {
		h := ev.(map[string]any)
		fmt.Println(h["callback_id"], h["kind"], h["value"])
	}
}
```

`Methods(recv)` lists every snake_case name `Call` accepts for a handle, and
`Call(recv, name, args...)` is the uniform dynamic entry point rbgo binds.

## License

BSD-3-Clause. See [LICENSE](LICENSE).
