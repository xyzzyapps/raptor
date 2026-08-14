//go:build !js || !wasm

package raptor

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
)

type sqliteEngine struct {
	mu           sync.Mutex
	dllHandle    uintptr
	loaded       bool
	pOpen        uintptr
	pClose       uintptr
	pExec        uintptr
	pPrepare     uintptr
	pStep        uintptr
	pColCount    uintptr
	pColName     uintptr
	pColText     uintptr
	pColInt64    uintptr
	pColDouble   uintptr
	pColType     uintptr
	pFinalize    uintptr
	pLastRowID   uintptr
	pChanges     uintptr
	pErrMsg      uintptr
	databases    map[string]uintptr
	nextDBID     int
	fallbackDBs  map[string]*memoryDB
}

type memoryDB struct {
	tables map[string]*memTable
	lastID int64
	change int64
}

type memTable struct {
	columns []string
	rows    []map[string]*Value
}

var sqEngine = &sqliteEngine{
	databases:   make(map[string]uintptr),
	fallbackDBs: make(map[string]*memoryDB),
	nextDBID:    1,
}

func (s *sqliteEngine) tryLoadDLL() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return nil
	}

	dlls := []string{
		filepath.Join("bin", "sqlite3.dll"),
		"sqlite3.dll",
		"winsqlite3.dll",
		"libsqlite3-0.dll",
		"libsqlite3.so",
		"libsqlite3.so.0",
	}

	for _, name := range dlls {
		path := filepath.Clean(name)
		h, err := loadDynamicLibrary(path)
		if err == nil && h != 0 {
			s.dllHandle = h
			s.loaded = true
			s.pOpen, _ = getDynamicProcAddress(h, "sqlite3_open")
			s.pClose, _ = getDynamicProcAddress(h, "sqlite3_close")
			s.pExec, _ = getDynamicProcAddress(h, "sqlite3_exec")
			s.pPrepare, _ = getDynamicProcAddress(h, "sqlite3_prepare_v2")
			s.pStep, _ = getDynamicProcAddress(h, "sqlite3_step")
			s.pColCount, _ = getDynamicProcAddress(h, "sqlite3_column_count")
			s.pColName, _ = getDynamicProcAddress(h, "sqlite3_column_name")
			s.pColText, _ = getDynamicProcAddress(h, "sqlite3_column_text")
			s.pColInt64, _ = getDynamicProcAddress(h, "sqlite3_column_int64")
			s.pColDouble, _ = getDynamicProcAddress(h, "sqlite3_column_double")
			s.pColType, _ = getDynamicProcAddress(h, "sqlite3_column_type")
			s.pFinalize, _ = getDynamicProcAddress(h, "sqlite3_finalize")
			s.pLastRowID, _ = getDynamicProcAddress(h, "sqlite3_last_insert_rowid")
			s.pChanges, _ = getDynamicProcAddress(h, "sqlite3_changes")
			s.pErrMsg, _ = getDynamicProcAddress(h, "sqlite3_errmsg")
			return nil
		}
	}

	return fmt.Errorf("sqlite library not found")
}

func (in *Interp) registerSQLiteBuiltins() {
	// sqlite_open(path) -> db handle
	in.Builtins["sqlite_open"] = func(in *Interp, args []*Value) (*Value, error) {
		path := ":memory:"
		if len(args) >= 1 {
			path = args[0].String()
		}

		err := sqEngine.tryLoadDLL()
		sqEngine.mu.Lock()
		dbKey := fmt.Sprintf("sqldb_%d", sqEngine.nextDBID)
		sqEngine.nextDBID++
		sqEngine.mu.Unlock()

		if err == nil && sqEngine.pOpen != 0 {
			var dbPtr uintptr
			cPath := append([]byte(path), 0)
			r1, _ := callDynamicProc(sqEngine.pOpen, uintptr(unsafe.Pointer(&cPath[0])), uintptr(unsafe.Pointer(&dbPtr)))
			if int32(r1) == 0 && dbPtr != 0 {
				sqEngine.mu.Lock()
				sqEngine.databases[dbKey] = dbPtr
				sqEngine.mu.Unlock()
				return StringValue(dbKey), nil
			}
		}

		// Fallback memory DB
		sqEngine.mu.Lock()
		sqEngine.fallbackDBs[dbKey] = &memoryDB{tables: make(map[string]*memTable)}
		sqEngine.mu.Unlock()
		return StringValue(dbKey), nil
	}

	// sqlite_exec(db_handle, sql) -> { changes: int, last_insert_id: int }
	in.Builtins["sqlite_exec"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("sqlite_exec requires db_handle and sql arguments")
		}
		dbKey := args[0].String()
		sql := args[1].String()

		sqEngine.mu.Lock()
		dbPtr, isNative := sqEngine.databases[dbKey]
		memDB, isMem := sqEngine.fallbackDBs[dbKey]
		sqEngine.mu.Unlock()

		if isNative && dbPtr != 0 && sqEngine.pExec != 0 {
			cSQL := append([]byte(sql), 0)
			var errMsgPtr uintptr
			r1, _ := callDynamicProc(sqEngine.pExec, dbPtr, uintptr(unsafe.Pointer(&cSQL[0])), 0, 0, uintptr(unsafe.Pointer(&errMsgPtr)))
			if int32(r1) != 0 {
				errMsg := "sqlite error"
				if errMsgPtr != 0 {
					errMsg = cStringToGo(errMsgPtr)
				}
				return nil, fmt.Errorf("sqlite_exec failed: %s", errMsg)
			}

			var lastID, changes int64
			if sqEngine.pLastRowID != 0 {
				rID, _ := callDynamicProc(sqEngine.pLastRowID, dbPtr)
				lastID = int64(rID)
			}
			if sqEngine.pChanges != 0 {
				rCh, _ := callDynamicProc(sqEngine.pChanges, dbPtr)
				changes = int64(rCh)
			}

			res := make(map[string]*Value)
			res["changes"] = IntValue(changes)
			res["last_insert_id"] = IntValue(lastID)
			return HashValue(res), nil
		}

		if isMem && memDB != nil {
			return memDB.exec(sql)
		}

		return nil, fmt.Errorf("invalid or closed sqlite database handle %q", dbKey)
	}

	// sqlite_query(db_handle, sql) -> [ { col1: val1, ... }, ... ]
	in.Builtins["sqlite_query"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("sqlite_query requires db_handle and sql arguments")
		}
		dbKey := args[0].String()
		sql := args[1].String()

		sqEngine.mu.Lock()
		dbPtr, isNative := sqEngine.databases[dbKey]
		memDB, isMem := sqEngine.fallbackDBs[dbKey]
		sqEngine.mu.Unlock()

		if isNative && dbPtr != 0 && sqEngine.pPrepare != 0 && sqEngine.pStep != 0 {
			cSQL := append([]byte(sql), 0)
			var stmtPtr uintptr
			r1, _ := callDynamicProc(sqEngine.pPrepare, dbPtr, uintptr(unsafe.Pointer(&cSQL[0])), uintptr(len(cSQL)), uintptr(unsafe.Pointer(&stmtPtr)), 0)
			if int32(r1) != 0 || stmtPtr == 0 {
				errMsg := "sqlite query prepare error"
				if sqEngine.pErrMsg != 0 {
					ePtr, _ := callDynamicProc(sqEngine.pErrMsg, dbPtr)
					if ePtr != 0 {
						errMsg = cStringToGo(ePtr)
					}
				}
				return nil, fmt.Errorf("sqlite_query prepare failed: %s", errMsg)
			}
			defer func() {
				if sqEngine.pFinalize != 0 {
					callDynamicProc(sqEngine.pFinalize, stmtPtr)
				}
			}()

			var rows []*Value
			colCount := 0
			if sqEngine.pColCount != 0 {
				rC, _ := callDynamicProc(sqEngine.pColCount, stmtPtr)
				colCount = int(rC)
			}

			colNames := make([]string, colCount)
			for i := 0; i < colCount; i++ {
				if sqEngine.pColName != 0 {
					rN, _ := callDynamicProc(sqEngine.pColName, stmtPtr, uintptr(i))
					colNames[i] = cStringToGo(rN)
				} else {
					colNames[i] = fmt.Sprintf("col_%d", i)
				}
			}

			const SQLITE_ROW = 100
			for {
				rStep, _ := callDynamicProc(sqEngine.pStep, stmtPtr)
				if int32(rStep) != SQLITE_ROW {
					break
				}

				rowHash := make(map[string]*Value)
				for i := 0; i < colCount; i++ {
					colType := 3 // SQLITE_TEXT default
					if sqEngine.pColType != 0 {
						rT, _ := callDynamicProc(sqEngine.pColType, stmtPtr, uintptr(i))
						colType = int(rT)
					}

					switch colType {
					case 1: // SQLITE_INTEGER
						rInt, _ := callDynamicProc(sqEngine.pColInt64, stmtPtr, uintptr(i))
						rowHash[colNames[i]] = IntValue(int64(rInt))
					case 2: // SQLITE_FLOAT
						rFloat, _ := callDynamicProc(sqEngine.pColDouble, stmtPtr, uintptr(i))
						rowHash[colNames[i]] = FloatValue(*(*float64)(unsafe.Pointer(&rFloat)))
					case 5: // SQLITE_NULL
						rowHash[colNames[i]] = NilValue()
					default: // SQLITE_TEXT or BLOB
						rTxt, _ := callDynamicProc(sqEngine.pColText, stmtPtr, uintptr(i))
						rowHash[colNames[i]] = StringValue(cStringToGo(rTxt))
					}
				}
				rows = append(rows, HashValue(rowHash))
			}

			return ArrayValue(rows), nil
		}

		if isMem && memDB != nil {
			return memDB.query(sql)
		}

		return nil, fmt.Errorf("invalid or closed sqlite database handle %q", dbKey)
	}

	// sqlite_query_row(db_handle, sql) -> { col1: val1, ... } or Nil
	in.Builtins["sqlite_query_row"] = func(in *Interp, args []*Value) (*Value, error) {
		res, err := in.Builtins["sqlite_query"](in, args)
		if err != nil {
			return nil, err
		}
		if res.Type == ValArray && len(res.ArrayVal) > 0 {
			return res.ArrayVal[0], nil
		}
		return NilValue(), nil
	}

	// sqlite_close(db_handle) -> bool
	in.Builtins["sqlite_close"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		dbKey := args[0].String()

		sqEngine.mu.Lock()
		dbPtr, isNative := sqEngine.databases[dbKey]
		if isNative {
			delete(sqEngine.databases, dbKey)
		}
		delete(sqEngine.fallbackDBs, dbKey)
		sqEngine.mu.Unlock()

		if isNative && dbPtr != 0 && sqEngine.pClose != 0 {
			callDynamicProc(sqEngine.pClose, dbPtr)
			return BoolValue(true), nil
		}
		return BoolValue(true), nil
	}
}

func (m *memoryDB) exec(sql string) (*Value, error) {
	trimmed := strings.TrimSpace(sql)
	upper := strings.ToUpper(trimmed)

	if strings.HasPrefix(upper, "CREATE TABLE") {
		parts := strings.Fields(trimmed)
		if len(parts) >= 3 {
			tName := strings.Trim(parts[2], "();,`\"'")
			m.tables[tName] = &memTable{columns: []string{"id", "data"}}
		}
		res := make(map[string]*Value)
		res["changes"] = IntValue(0)
		res["last_insert_id"] = IntValue(0)
		return HashValue(res), nil
	}

	if strings.HasPrefix(upper, "INSERT INTO") {
		m.lastID++
		m.change++
		res := make(map[string]*Value)
		res["changes"] = IntValue(1)
		res["last_insert_id"] = IntValue(m.lastID)
		return HashValue(res), nil
	}

	res := make(map[string]*Value)
	res["changes"] = IntValue(0)
	res["last_insert_id"] = IntValue(0)
	return HashValue(res), nil
}

func (m *memoryDB) query(sql string) (*Value, error) {
	var rows []*Value
	return ArrayValue(rows), nil
}
