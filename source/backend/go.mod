module pairingeasy

go 1.27

replace golang.org/x/sys => ./third_party/x-sys

require github.com/ncruces/go-sqlite3 v0.36.0

require (
	github.com/ncruces/go-sqlite3-wasm/v6 v6.4.35401 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)
