//go:build !js || !wasm

package raptor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
		`C:\Windows\System32\winsqlite3.dll`,
		`C:\msys64\ucrt64\bin\libsqlite3-0.dll`,
		"libsqlite3.so",
		"libsqlite3.so.0",
		"libsqlite3.dylib",
	}
	if sysRoot := os.Getenv("SystemRoot"); sysRoot != "" {
		dlls = append(dlls, filepath.Join(sysRoot, "System32", "winsqlite3.dll"))
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

var (
	reCreateTable = regexp.MustCompile(`(?is)^CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["'\x60]?(\w+)["'\x60]?\s*\((.+)\)`)
	reDropTable   = regexp.MustCompile(`(?is)^DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?["'\x60]?(\w+)["'\x60]?`)
	reInsertInto  = regexp.MustCompile(`(?is)^INSERT\s+INTO\s+["'\x60]?(\w+)["'\x60]?\s*(?:\(([^)]+)\))?\s*VALUES\s*\((.+)\)`)
	reDeleteFrom  = regexp.MustCompile(`(?is)^DELETE\s+FROM\s+["'\x60]?(\w+)["'\x60]?(?:\s+WHERE\s+(.+))?`)
	reUpdateTable = regexp.MustCompile(`(?is)^UPDATE\s+["'\x60]?(\w+)["'\x60]?\s+SET\s+(.+?)(?:\s+WHERE\s+(.+))?$`)
	reSelectFrom  = regexp.MustCompile(`(?is)^SELECT\s+(.+?)\s+FROM\s+["'\x60]?(\w+)["'\x60]?(?:\s+WHERE\s+(.+?))?(?:\s+ORDER\s+BY\s+(.+?))?(?:\s+LIMIT\s+(\d+))?$`)
)

func parseSQLLiteral(raw string) *Value {
	s := strings.TrimSpace(raw)
	if strings.EqualFold(s, "NULL") {
		return NilValue()
	}
	if len(s) >= 2 && ((s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"')) {
		inner := s[1 : len(s)-1]
		inner = strings.ReplaceAll(inner, "''", "'")
		return StringValue(inner)
	}
	if iv, err := strconv.ParseInt(s, 10, 64); err == nil {
		return IntValue(iv)
	}
	if fv, err := strconv.ParseFloat(s, 64); err == nil {
		return FloatValue(fv)
	}
	return StringValue(s)
}

func splitSQLCSV(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := byte(0)
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote != 0 {
			cur.WriteByte(c)
			if c == inQuote {
				if i+1 < len(s) && s[i+1] == inQuote {
					cur.WriteByte(s[i+1])
					i++
				} else {
					inQuote = 0
				}
			}
			continue
		}
		if c == '\'' || c == '"' {
			inQuote = c
			cur.WriteByte(c)
			continue
		}
		if c == '(' {
			depth++
		} else if c == ')' {
			depth--
		} else if c == ',' && depth == 0 {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if cur.Len() > 0 {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}

func matchSQLWhere(row map[string]*Value, where string) bool {
	where = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(where), ";"))
	if where == "" {
		return true
	}
	for _, cond := range regexp.MustCompile(`(?i)\s+AND\s+`).Split(where, -1) {
		cond = strings.TrimSpace(cond)
		for _, op := range []string{"!=", "<>", "<=", ">=", "=", "<", ">"} {
			if idx := strings.Index(cond, op); idx > 0 {
				col := strings.Trim(strings.TrimSpace(cond[:idx]), "`\"'")
				rhs := parseSQLLiteral(cond[idx+len(op):])
				lv, ok := row[col]
				if !ok || lv == nil {
					lv = NilValue()
				}
				switch op {
				case "=":
					if lv.String() != rhs.String() {
						return false
					}
				case "!=", "<>":
					if lv.String() == rhs.String() {
						return false
					}
				case "<":
					lf, _ := strconv.ParseFloat(lv.String(), 64)
					rf, _ := strconv.ParseFloat(rhs.String(), 64)
					if !(lf < rf) {
						return false
					}
				case "<=":
					lf, _ := strconv.ParseFloat(lv.String(), 64)
					rf, _ := strconv.ParseFloat(rhs.String(), 64)
					if !(lf <= rf) {
						return false
					}
				case ">":
					lf, _ := strconv.ParseFloat(lv.String(), 64)
					rf, _ := strconv.ParseFloat(rhs.String(), 64)
					if !(lf > rf) {
						return false
					}
				case ">=":
					lf, _ := strconv.ParseFloat(lv.String(), 64)
					rf, _ := strconv.ParseFloat(rhs.String(), 64)
					if !(lf >= rf) {
						return false
					}
				}
				break
			}
		}
	}
	return true
}

func (m *memoryDB) exec(sql string) (*Value, error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sql), ";"))
	if mc := reCreateTable.FindStringSubmatch(trimmed); mc != nil {
		tName := strings.ToLower(mc[1])
		if _, exists := m.tables[tName]; !exists {
			var cols []string
			for _, colDef := range splitSQLCSV(mc[2]) {
				fields := strings.Fields(colDef)
				if len(fields) > 0 {
					cName := strings.Trim(fields[0], "`\"'")
					upperC := strings.ToUpper(cName)
					if upperC != "PRIMARY" && upperC != "UNIQUE" && upperC != "FOREIGN" && upperC != "CHECK" {
						cols = append(cols, cName)
					}
				}
			}
			m.tables[tName] = &memTable{columns: cols}
		}
		return HashValue(map[string]*Value{"changes": IntValue(0), "last_insert_id": IntValue(m.lastID)}), nil
	}
	if md := reDropTable.FindStringSubmatch(trimmed); md != nil {
		delete(m.tables, strings.ToLower(md[1]))
		return HashValue(map[string]*Value{"changes": IntValue(0), "last_insert_id": IntValue(m.lastID)}), nil
	}
	if mi := reInsertInto.FindStringSubmatch(trimmed); mi != nil {
		tName := strings.ToLower(mi[1])
		tbl, ok := m.tables[tName]
		if !ok {
			tbl = &memTable{}
			m.tables[tName] = tbl
		}
		var cols []string
		if strings.TrimSpace(mi[2]) != "" {
			for _, c := range splitSQLCSV(mi[2]) {
				cols = append(cols, strings.Trim(c, "`\"' "))
			}
		} else {
			cols = tbl.columns
		}
		vals := splitSQLCSV(mi[3])
		m.lastID++
		row := make(map[string]*Value)
		for i, col := range cols {
			if i < len(vals) {
				row[col] = parseSQLLiteral(vals[i])
			}
		}
		if _, hasID := row["id"]; !hasID {
			for _, c := range tbl.columns {
				if strings.EqualFold(c, "id") {
					row[c] = IntValue(m.lastID)
				}
			}
		}
		tbl.rows = append(tbl.rows, row)
		m.change = 1
		return HashValue(map[string]*Value{"changes": IntValue(1), "last_insert_id": IntValue(m.lastID)}), nil
	}
	if mu := reUpdateTable.FindStringSubmatch(trimmed); mu != nil {
		tName := strings.ToLower(mu[1])
		tbl, ok := m.tables[tName]
		if !ok {
			return HashValue(map[string]*Value{"changes": IntValue(0), "last_insert_id": IntValue(m.lastID)}), nil
		}
		assigns := splitSQLCSV(mu[2])
		where := mu[3]
		var chg int64
		for _, r := range tbl.rows {
			if matchSQLWhere(r, where) {
				for _, as := range assigns {
					if eq := strings.IndexByte(as, '='); eq > 0 {
						col := strings.Trim(strings.TrimSpace(as[:eq]), "`\"'")
						r[col] = parseSQLLiteral(as[eq+1:])
					}
				}
				chg++
			}
		}
		return HashValue(map[string]*Value{"changes": IntValue(chg), "last_insert_id": IntValue(m.lastID)}), nil
	}
	if mdel := reDeleteFrom.FindStringSubmatch(trimmed); mdel != nil {
		tName := strings.ToLower(mdel[1])
		tbl, ok := m.tables[tName]
		if !ok {
			return HashValue(map[string]*Value{"changes": IntValue(0), "last_insert_id": IntValue(m.lastID)}), nil
		}
		where := mdel[2]
		var kept []map[string]*Value
		var chg int64
		for _, r := range tbl.rows {
			if matchSQLWhere(r, where) {
				chg++
			} else {
				kept = append(kept, r)
			}
		}
		tbl.rows = kept
		return HashValue(map[string]*Value{"changes": IntValue(chg), "last_insert_id": IntValue(m.lastID)}), nil
	}
	return HashValue(map[string]*Value{"changes": IntValue(0), "last_insert_id": IntValue(m.lastID)}), nil
}

func (m *memoryDB) query(sql string) (*Value, error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sql), ";"))
	ms := reSelectFrom.FindStringSubmatch(trimmed)
	if ms == nil {
		return ArrayValue(nil), nil
	}
	selCols := strings.TrimSpace(ms[1])
	tName := strings.ToLower(ms[2])
	where := ms[3]
	limitStr := ms[5]
	tbl, ok := m.tables[tName]
	if !ok {
		return ArrayValue(nil), nil
	}
	var matched []map[string]*Value
	for _, r := range tbl.rows {
		if matchSQLWhere(r, where) {
			matched = append(matched, r)
		}
	}
	if strings.HasPrefix(strings.ToUpper(selCols), "COUNT(") {
		colKey := selCols
		if idx := strings.Index(strings.ToUpper(selCols), " AS "); idx > 0 {
			colKey = strings.Trim(strings.TrimSpace(selCols[idx+4:]), "`\"'")
		}
		return ArrayValue([]*Value{HashValue(map[string]*Value{colKey: IntValue(int64(len(matched)))})}), nil
	}
	limit := len(matched)
	if limitStr != "" {
		if lv, err := strconv.Atoi(limitStr); err == nil && lv < limit {
			limit = lv
		}
	}
	var out []*Value
	for i := 0; i < limit; i++ {
		cp := make(map[string]*Value)
		if selCols == "*" {
			for k, v := range matched[i] {
				cp[k] = v
			}
		} else {
			for _, c := range splitSQLCSV(selCols) {
				cn := strings.Trim(c, "`\"' ")
				if v, exists := matched[i][cn]; exists {
					cp[cn] = v
				} else {
					cp[cn] = NilValue()
				}
			}
		}
		out = append(out, HashValue(cp))
	}
	return ArrayValue(out), nil
}
