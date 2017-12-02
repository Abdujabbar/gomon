package sql

import (
	"context"
	stdsql "database/sql"
	"database/sql/driver"

	"github.com/iahmedov/gomon"
)

type PluginConfig struct {
	MaxRows  int
	QueryLen int
}

type wrappedDriver struct {
	parent driver.Driver
	c      *PluginConfig
}

type wrappedConn struct {
	parent driver.Conn
	c      *PluginConfig
	et     gomon.EventTracker
}

type wrappedRows struct {
	parent driver.Rows
	c      *PluginConfig
	et     gomon.EventTracker
}

type wrappedStmt struct {
	// TODO: since database/sql makes it impossible to know
	// whether stmt is from transaction or connection, we need
	// to store last created transaction in wrappedConn, so that
	// we can create parent/child relationship
	// between Tx/Stmt and Conn/Stmt
	parent driver.Stmt
	c      *PluginConfig
	et     gomon.EventTracker
}

type wrappedTx struct {
	parent driver.Tx
	c      *PluginConfig
	et     gomon.EventTracker
}

type wrappedResult struct {
	parent driver.Result
	c      *PluginConfig
	et     gomon.EventTracker
}

var _ driver.Driver = (*wrappedDriver)(nil)

var _ driver.Conn = (*wrappedConn)(nil)
var _ driver.ConnBeginTx = (*wrappedConn)(nil)
var _ driver.ConnPrepareContext = (*wrappedConn)(nil)

var _ driver.Rows = (*wrappedRows)(nil)
var _ driver.Stmt = (*wrappedStmt)(nil)
var _ driver.StmtExecContext = (*wrappedStmt)(nil)
var _ driver.StmtQueryContext = (*wrappedStmt)(nil)
var _ driver.Tx = (*wrappedTx)(nil)
var _ driver.Result = (*wrappedResult)(nil)

var defaultConfig = &PluginConfig{
	MaxRows:  10,
	QueryLen: 1024,
}

var (
	pluginName     = "gomon/sql"
	KeyQuery       = pluginName + ":query"
	KeyParams      = pluginName + ":params"
	KeyNamedParams = pluginName + ":named_params"
)

func MonitoringDriver(d driver.Driver) driver.Driver {
	return &wrappedDriver{
		parent: d,
		c:      defaultConfig,
	}
}

func (wdr *wrappedDriver) Open(name string) (conn driver.Conn, err error) {
	defer func() {
		if err != nil {
			et := gomon.FromContext(nil).NewChild(false)
			et.AddError(err)
			et.SetFingerprint("driver-open")
			et.Set("driver-name", name)
			et.Finish()
		}
	}()

	conn, err = wdr.parent.Open(name)
	if err != nil {
		conn = &wrappedConn{
			parent: conn,
			c:      wdr.c,
			et:     gomon.FromContext(nil).NewChild(false),
		}
	} else {
		conn = nil
	}
	return
}

func (wcn *wrappedConn) Prepare(query string) (stmt driver.Stmt, err error) {
	return wcn.PrepareContext(context.Background(), query)
}

func (wcn *wrappedConn) PrepareContext(ctx context.Context, query string) (stmt driver.Stmt, err error) {
	et := wcn.et.NewChild(false)
	et.SetFingerprint("conn-prepare")
	et.Set("query", query)

	defer func() {
		if err != nil {
			et.AddError(err)
			et.Finish()
		}
	}()

	if parentPrepCtx, ok := wcn.parent.(driver.ConnPrepareContext); ok {
		stmt, err = parentPrepCtx.PrepareContext(ctx, query)
	} else {
		stmt, err = wcn.parent.Prepare(query)
	}

	stmt = &wrappedStmt{
		parent: stmt,
		c:      wcn.c,
		et:     et,
	}
	return
}

func (wcn *wrappedConn) Close() (err error) {
	err = wcn.parent.Close()
	if err != nil {
		et := wcn.et.NewChild(true)
		et.AddError(err)
		et.SetFingerprint("conn-close")
		et.Finish()
	}
	wcn.et.Finish()

	return
}

func (wcn *wrappedConn) Begin() (tx driver.Tx, err error) {
	// return nil, driver.ErrSkip
	isolation := driver.IsolationLevel(stdsql.LevelDefault)
	return wcn.BeginTx(context.Background(), driver.TxOptions{isolation, false})
}

func (wcn *wrappedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (tx driver.Tx, err error) {
	et := wcn.et.NewChild(false)
	et.SetFingerprint("conn-begintx")
	defer func() {
		if err != nil {
			et.AddError(err)
			et.Finish()
		}
	}()

	if parentBeginTx, ok := wcn.parent.(driver.ConnBeginTx); ok {
		tx, err = parentBeginTx.BeginTx(gomon.ContextWith(ctx, et), opts)
	} else {
		tx, err = wcn.parent.Begin()
	}

	if err != nil {
		return nil, err
	}

	return &wrappedTx{tx, wcn.c, et}, nil
}

func (wrs *wrappedRows) Columns() (cols []string) {
	cols = wrs.parent.Columns()
	et := wrs.et.NewChild(true)
	et.SetFingerprint("sql-wrows-columns")
	et.Set("columns", cols)
	et.Finish()
	return
}

func (wrs *wrappedRows) Close() (err error) {
	err = wrs.parent.Close()
	if err != nil {
		et := wrs.et.NewChild(true)
		et.SetFingerprint("sql-wrows-close")
		et.AddError(err)
		et.Finish()
	}
	wrs.et.Finish()
	return
}

func (wrs *wrappedRows) Next(dest []driver.Value) (err error) {
	err = wrs.parent.Next(dest)
	if err != nil {
		et := wrs.et.NewChild(true)
		et.SetFingerprint("sql-wrows-next")
		et.AddError(err)
		et.Finish()
	} else {
		rows := wrs.et.Get("rows").([][]driver.Value)
		if rows == nil {
			rows = make([][]driver.Value, 0, wrs.c.MaxRows)
		}

		if len(rows) < cap(rows) {
			rows = append(rows, dest)
		}
		wrs.et.Set("rows", rows)
	}
	return
}

func (wst *wrappedStmt) Close() (err error) {
	err = wst.parent.Close()
	if err != nil {
		et := wst.et.NewChild(true)
		et.AddError(err)
		et.SetFingerprint("sql-wstmt-close")
		et.Finish()
	}
	wst.et.Finish()
	return
}

func (wst *wrappedStmt) NumInput() int {
	return wst.parent.NumInput()
}

func (wst *wrappedStmt) Exec(args []driver.Value) (res driver.Result, err error) {
	et := wst.et.NewChild(false)
	et.SetFingerprint("sql-wstmt-exec")
	et.Start()

	res, err = wst.parent.Exec(args)

	if err != nil {
		et.AddError(err)
	} else {
		// TODO: should we populate data here?
		// make it configurable
		lid, errl := res.LastInsertId()
		if errl != nil {
			et.Set("last-id", lid)
		} else {
			et.Set("last-id-err", errl)
		}

		raf, errf := res.RowsAffected()
		if errf != nil {
			et.Set("rows-aff", raf)
		} else {
			et.Set("rows-aff-err", errf)
		}
	}
	et.Finish()
	return
}

func (wst *wrappedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (res driver.Result, err error) {
	et := wst.et.NewChild(false)
	et.SetFingerprint("sql-wstmt-execctx")
	et.Start()

	if parentExecCtx, ok := wst.parent.(driver.StmtExecContext); ok {
		res, err = parentExecCtx.ExecContext(ctx, args)
	} else {
		// not sure about handling column names and positions yet
		res, err = nil, driver.ErrSkip
	}

	if err != nil {
		et.AddError(err)
	} else {
		// TODO: should we populate data here?
		// make it configurable
		lid, errl := res.LastInsertId()
		if errl != nil {
			et.Set("last-id", lid)
		} else {
			et.Set("last-id-err", errl)
		}

		raf, errf := res.RowsAffected()
		if errf != nil {
			et.Set("rows-aff", raf)
		} else {
			et.Set("rows-aff-err", errf)
		}
	}
	et.Finish()
	return
}

func (wst *wrappedStmt) Query(args []driver.Value) (rows driver.Rows, err error) {
	et := wst.et.NewChild(false)
	et.SetFingerprint("sql-wstmt-query")
	et.Start()

	rows, err = wst.parent.Query(args)

	if err != nil {
		et.AddError(err)
	} else {
		rows = &wrappedRows{
			parent: rows,
			c:      wst.c,
			et:     et,
		}
	}
	// NOTE: this creates double entry in database
	// 1. for query execution time
	// 2. after rows.Close() called
	et.Finish()
	return
}

func (wst *wrappedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (rows driver.Rows, err error) {
	et := wst.et.NewChild(false)
	et.SetFingerprint("sql-wstmt-queryctx")
	et.Start()

	if parentQueryCtx, ok := wst.parent.(driver.StmtQueryContext); ok {
		rows, err = parentQueryCtx.QueryContext(ctx, args)
	} else {
		// not sure about handling column names and positions yet
		rows, err = nil, driver.ErrSkip
	}

	if err != nil {
		et.AddError(err)
	} else {
		rows = &wrappedRows{
			parent: rows,
			c:      wst.c,
			et:     et,
		}
	}
	// NOTE: this creates double entry in database
	// 1. for query execution time
	// 2. after rows.Close() called (with data)
	et.Finish()
	return
}

func (wtx *wrappedTx) Commit() (err error) {
	err = wtx.parent.Commit()

	et := wtx.et.NewChild(true)
	et.SetFingerprint("sql-wtx-commit")
	if err != nil {
		et.AddError(err)
	}
	et.Finish()
	wtx.et.Finish()
	return
}

func (wtx *wrappedTx) Rollback() (err error) {
	err = wtx.parent.Rollback()

	et := wtx.et.NewChild(true)
	et.SetFingerprint("sql-wtx-rollback")
	if err != nil {
		et.AddError(err)
	}
	et.Finish()
	wtx.et.Finish()
	return
}

func (wrs *wrappedResult) LastInsertId() (id int64, err error) {
	return wrs.parent.LastInsertId()
}

func (wrs *wrappedResult) RowsAffected() (n int64, err error) {
	return wrs.parent.RowsAffected()
}
