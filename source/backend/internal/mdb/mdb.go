// Package mdb is a minimal, read-only reader for Microsoft Access Jet4
// (Access 2000/2003) databases. It implements just enough of the format
// (catalog, table definitions, data pages, fixed and variable columns,
// compressed Unicode text) to read the FFE "PapiData" Data.mdb file.
//
// Format reference: mdbtools HACKING notes. Memo/OLE columns are returned as
// nil; overflow ("lookup") rows and deleted rows are skipped.
package mdb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	pageSize = 4096

	pageTypeData = 0x01
	pageTypeTdef = 0x02

	offsetMask = 0x1fff
	flagDelete = 0x4000
	flagLookup = 0x8000
)

// Column types (Jet4).
const (
	TypeBool     = 0x01
	TypeByte     = 0x02
	TypeInt      = 0x03
	TypeLong     = 0x04
	TypeMoney    = 0x05
	TypeFloat    = 0x06
	TypeDouble   = 0x07
	TypeDateTime = 0x08
	TypeBinary   = 0x09
	TypeText     = 0x0A
	TypeOLE      = 0x0B
	TypeMemo     = 0x0C
	TypeGUID     = 0x0F
	TypeNumeric  = 0x10
)

var ErrNotJet4 = errors.New("mdb: only Jet4 (Access 2000/2003) files are supported")

// DB is an open Access database.
type DB struct {
	r      io.ReaderAt
	closer io.Closer
	pages  int64
	tables map[string]uint32 // table name -> tdef page
}

// Column describes one table column.
type Column struct {
	Name        string
	Type        byte
	Num         int // col_num, used for the null bitmap
	VarNum      int
	FixedOffset int
	Size        int
	Fixed       bool
}

// Table is a parsed table definition.
type Table struct {
	Name       string
	tdefPage   uint32
	NumRows    int
	NumVarCols int
	Columns    []Column
}

// Open opens an .mdb file from disk.
func Open(path string) (*DB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	db, err := New(f, st.Size())
	if err != nil {
		f.Close()
		return nil, err
	}
	db.closer = f
	return db, nil
}

// New reads a database from any ReaderAt of the given size.
func New(r io.ReaderAt, size int64) (*DB, error) {
	head := make([]byte, 0x20)
	if _, err := r.ReadAt(head, 0); err != nil {
		return nil, fmt.Errorf("mdb: read header: %w", err)
	}
	if string(head[4:19]) != "Standard Jet DB" {
		return nil, errors.New("mdb: not a Jet database")
	}
	if head[0x14] != 0x01 {
		return nil, ErrNotJet4
	}
	db := &DB{r: r, pages: size / pageSize, tables: map[string]uint32{}}
	if err := db.loadCatalog(); err != nil {
		return nil, err
	}
	return db, nil
}

// Close releases the underlying file if Open was used.
func (db *DB) Close() error {
	if db.closer != nil {
		return db.closer.Close()
	}
	return nil
}

// TableNames returns user table names.
func (db *DB) TableNames() []string {
	out := make([]string, 0, len(db.tables))
	for n := range db.tables {
		out = append(out, n)
	}
	return out
}

func (db *DB) page(n uint32) ([]byte, error) {
	buf := make([]byte, pageSize)
	if _, err := db.r.ReadAt(buf, int64(n)*pageSize); err != nil {
		return nil, fmt.Errorf("mdb: read page %d: %w", n, err)
	}
	return buf, nil
}

func (db *DB) loadCatalog() error {
	sys, err := db.readTable("MSysObjects", 2)
	if err != nil {
		return fmt.Errorf("mdb: catalog: %w", err)
	}
	return db.Scan(sys, func(row map[string]any) error {
		name, _ := row["Name"].(string)
		typ, _ := row["Type"].(int64)
		id, _ := row["Id"].(int64)
		flags, _ := row["Flags"].(int64)
		if typ == 1 && name != "" && uint32(flags)&0x80000002 == 0 {
			db.tables[name] = uint32(id) & 0x00ffffff
		}
		return nil
	})
}

// Table returns the definition of a user table.
func (db *DB) Table(name string) (*Table, error) {
	pg, ok := db.tables[name]
	if !ok {
		for n, p := range db.tables {
			if strings.EqualFold(n, name) {
				pg, ok, name = p, true, n
				break
			}
		}
	}
	if !ok {
		return nil, fmt.Errorf("mdb: table %q not found", name)
	}
	return db.readTable(name, pg)
}

func (db *DB) readTable(name string, tdefPage uint32) (*Table, error) {
	// A table definition may span several pages; continuation pages carry
	// an 8-byte header that is not part of the definition.
	first, err := db.page(tdefPage)
	if err != nil {
		return nil, err
	}
	if first[0] != pageTypeTdef {
		return nil, fmt.Errorf("mdb: page %d is not a table definition", tdefPage)
	}
	buf := append([]byte{}, first...)
	next := binary.LittleEndian.Uint32(first[4:])
	for guard := 0; next != 0 && guard < 1024; guard++ {
		p, err := db.page(next)
		if err != nil {
			return nil, err
		}
		buf = append(buf, p[8:]...)
		next = binary.LittleEndian.Uint32(p[4:])
	}

	t := &Table{Name: name, tdefPage: tdefPage}
	t.NumRows = int(binary.LittleEndian.Uint32(buf[16:]))
	t.NumVarCols = int(binary.LittleEndian.Uint16(buf[43:]))
	numCols := int(binary.LittleEndian.Uint16(buf[45:]))
	numRealIdx := int(binary.LittleEndian.Uint32(buf[51:]))

	pos := 63 + numRealIdx*12
	cols := make([]Column, numCols)
	for i := range cols {
		e := buf[pos : pos+25]
		cols[i] = Column{
			Type:        e[0],
			Num:         int(binary.LittleEndian.Uint16(e[5:])),
			VarNum:      int(binary.LittleEndian.Uint16(e[7:])),
			Fixed:       e[15]&0x01 != 0,
			FixedOffset: int(binary.LittleEndian.Uint16(e[21:])),
			Size:        int(binary.LittleEndian.Uint16(e[23:])),
		}
		pos += 25
	}
	for i := range cols {
		l := int(binary.LittleEndian.Uint16(buf[pos:]))
		pos += 2
		cols[i].Name = decodeText(buf[pos : pos+l])
		pos += l
	}
	// Order by col_num, as the row layout does.
	for i := 1; i < len(cols); i++ {
		for j := i; j > 0 && cols[j-1].Num > cols[j].Num; j-- {
			cols[j-1], cols[j] = cols[j], cols[j-1]
		}
	}
	t.Columns = cols
	return t, nil
}

// Scan calls fn for each live row of the table, in page order. Returning an
// error from fn stops the scan and returns that error.
func (db *DB) Scan(t *Table, fn func(row map[string]any) error) error {
	for p := int64(1); p < db.pages; p++ {
		buf, err := db.page(uint32(p))
		if err != nil {
			return err
		}
		if buf[0] != pageTypeData || binary.LittleEndian.Uint32(buf[4:]) != t.tdefPage {
			continue
		}
		n := int(binary.LittleEndian.Uint16(buf[12:]))
		for i := 0; i < n; i++ {
			raw := binary.LittleEndian.Uint16(buf[14+2*i:])
			if raw&(flagDelete|flagLookup) != 0 {
				continue
			}
			start := int(raw & offsetMask)
			end := pageSize - 1
			if i > 0 {
				end = int(binary.LittleEndian.Uint16(buf[14+2*(i-1):])&offsetMask) - 1
			}
			if start <= 0 || end <= start || end >= pageSize {
				continue
			}
			row, err := crackRow(t, buf, start, end)
			if err != nil {
				return fmt.Errorf("mdb: page %d row %d: %w", p, i, err)
			}
			if err := fn(row); err != nil {
				return err
			}
		}
	}
	return nil
}

// crackRow decodes a row stored in buf[start:end] (end inclusive).
func crackRow(t *Table, buf []byte, start, end int) (row map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("malformed row: %v", r)
		}
	}()
	rowCols := int(binary.LittleEndian.Uint16(buf[start:]))
	bitmaskSz := (rowCols + 7) / 8
	nullmask := buf[end-bitmaskSz+1 : end+1]

	rowVarCols := 0
	var varOffsets []int
	if t.NumVarCols > 0 {
		rowVarCols = int(binary.LittleEndian.Uint16(buf[end-bitmaskSz-1:]))
		varOffsets = make([]int, rowVarCols+1)
		for i := range varOffsets {
			varOffsets[i] = int(binary.LittleEndian.Uint16(buf[end-bitmaskSz-3-2*i:]))
		}
	}
	rowFixedCols := rowCols - rowVarCols
	fixedFound := 0

	row = make(map[string]any, len(t.Columns))
	for _, c := range t.Columns {
		bit := c.Num
		notNull := bit/8 < len(nullmask) && nullmask[bit/8]&(1<<(bit%8)) != 0
		if c.Type == TypeBool {
			row[c.Name] = notNull
			if c.Fixed {
				fixedFound++
			}
			continue
		}
		var data []byte
		if c.Fixed && fixedFound < rowFixedCols {
			s := start + 2 + c.FixedOffset
			data = buf[s : s+c.Size]
			fixedFound++
		} else if !c.Fixed && c.VarNum < rowVarCols {
			s := varOffsets[c.VarNum]
			e := varOffsets[c.VarNum+1]
			if e < s {
				e = s
			}
			data = buf[start+s : start+e]
		} else {
			notNull = false
		}
		if !notNull {
			row[c.Name] = nil
			continue
		}
		row[c.Name] = decodeValue(c.Type, data)
	}
	return row, nil
}

func decodeValue(typ byte, d []byte) any {
	switch typ {
	case TypeByte:
		return int64(d[0])
	case TypeInt:
		return int64(int16(binary.LittleEndian.Uint16(d)))
	case TypeLong:
		return int64(int32(binary.LittleEndian.Uint32(d)))
	case TypeMoney:
		return float64(int64(binary.LittleEndian.Uint64(d))) / 10000
	case TypeFloat:
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(d)))
	case TypeDouble:
		return math.Float64frombits(binary.LittleEndian.Uint64(d))
	case TypeDateTime:
		days := math.Float64frombits(binary.LittleEndian.Uint64(d))
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		return base.Add(time.Duration(days * 24 * float64(time.Hour))).Round(time.Second)
	case TypeText:
		return decodeText(d)
	case TypeBinary, TypeGUID:
		return append([]byte{}, d...)
	default: // memo, OLE, numeric: not needed for PAPI
		return nil
	}
}

// decodeText decodes Jet4 text: UTF-16LE, or "compressed" when prefixed by
// 0xFF 0xFE (runs of single-byte chars, toggled by 0x00 bytes).
func decodeText(d []byte) string {
	if len(d) >= 2 && d[0] == 0xff && d[1] == 0xfe {
		var u []uint16
		compressed := true
		for i := 2; i < len(d); {
			if d[i] == 0x00 {
				compressed = !compressed
				i++
				continue
			}
			if compressed {
				u = append(u, uint16(d[i]))
				i++
			} else {
				if i+1 >= len(d) {
					break
				}
				u = append(u, binary.LittleEndian.Uint16(d[i:]))
				i += 2
			}
		}
		return string(utf16.Decode(u))
	}
	u := make([]uint16, len(d)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(d[2*i:])
	}
	return string(utf16.Decode(u))
}
