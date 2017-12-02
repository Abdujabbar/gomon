package sql

import (
	"context"
	"database/sql/driver"

	"github.com/iahmedov/gomon/plugin"
)

type PluginConfig struct {
	MaxRows  int
	QueryLen int
}

type sqlPlugin struct {
	config   *PluginConfig
	listener plugin.Listener
}

type wrappedDriver struct {
	parent driver.Driver
	p      *sqlPlugin
	c      *PluginConfig
}

type wrappedConn struct {
	parent driver.Conn
	p      *sqlPlugin
	c      *PluginConfig
	et     plugin.EventTracker
}

type wrappedRows struct {
	parent driver.Rows
	p      *sqlPlugin
	c      *PluginConfig
	et     plugin.EventTracker
}

type wrappedStmt struct {
	parent driver.Stmt
	p      *sqlPlugin
	c      *PluginConfig
	et     plugin.EventTracker
}

type wrappedTx struct {
	parent driver.Tx
	p      *sqlPlugin
	c      *PluginConfig
	et     plugin.EventTracker
}

type wrappedResult struct {
	parent driver.Result
	p      *sqlPlugin
	c      *PluginConfig
	et     plugin.EventTracker
}

var _ plugin.Plugin = (*sqlPlugin)(nil)
var _ driver.Driver = (*wrappedDriver)(nil)

var _ driver.Conn = (*wrappedConn)(nil)
var _ driver.ConnBeginTx = (*wrappedConn)(nil)

var _ driver.Rows = (*wrappedRows)(nil)
var _ driver.Stmt = (*wrappedStmt)(nil)
var _ driver.Tx = (*wrappedTx)(nil)
var _ driver.Result = (*wrappedResult)(nil)

var defaultPluginConfig = &PluginConfig{
	MaxRows:  10,
	QueryLen: 1024,
}

var defaultPlugin = &sqlPlugin{
	config: defaultPluginConfig,
}

var (
	pluginName     = "gomon/sql"
	KeyQuery       = pluginName + ":query"
	KeyParams      = pluginName + ":params"
	KeyNamedParams = pluginName + ":named_params"
)

func (s *sqlPlugin) Name() string {
	return pluginName
}

func (s *sqlPlugin) SetEventReceiver(l plugin.Listener) {
	s.listener = l
}

func (s *sqlPlugin) HandleTracker(et plugin.EventTracker) {
	s.listener.Feed(s.Name(), et)
}

func MonitoringDriver(d driver.Driver) driver.Driver {
	return &wrappedDriver{
		parent: d,
		p:      defaultPlugin,
		c:      defaultPlugin.config,
	}
}

func (wdr *wrappedDriver) Open(name string) (conn driver.Conn, err error) {
	defer func() {
		if err != nil {
			et := plugin.FromContext(nil, false, wdr.p)
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
			p:      wdr.p,
			c:      wdr.c,
		}
	}
	return
}

func (wcn *wrappedConn) Prepare(query string) (stmt driver.Stmt, err error) {
	return wcn.PrepareContext(context.Background(), query)
}

func (wcn *wrappedConn) PrepareContext(ctx context.Context, query string) (stmt driver.Stmt, err error) {
	et := plugin.FromContext(ctx, false, wcn.p)
	et.SetFingerprint("conn-prepare")
	et.Set("conn-preparectx", query)

	defer func() {
		if err != nil {
			et.AddError(err)
			et.Finish()
		}
	}()

	stmt, err = wcn.parent.Prepare(query)
	stmt = &wrappedStmt{
		parent: stmt,
		p:      wcn.p,
		c:      wcn.c,
		et:     et,
	}
	return
}

func (wcn *wrappedConn) Close() (err error) {
	err = wcn.parent.Close()
	if err != nil {
		et := plugin.FromContext(nil, false, wcn.p)
		et.AddError(err)
		et.SetFingerprint("conn-close")
		et.Finish()
	}

	return
}

func (wcn *wrappedConn) Begin() (tx driver.Tx, err error) {
	return nil, driver.ErrSkip
	// return wcn.BeginTx(context.Background(), driver.TxOptions{sql.LevelDefault, false})
}

func (wcn *wrappedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (tx driver.Tx, err error) {
	et := plugin.FromContext(ctx, false, wcn.p)
	et.SetFingerprint("conn-begintx")
	defer func() {
		if err != nil {
			et.AddError(err)
			et.Finish()
		}
	}()

	if parentBeginTx, ok := wcn.parent.(driver.ConnBeginTx); ok {
		tx, err = parentBeginTx.BeginTx(plugin.ContextWith(ctx, et), opts)
	} else {
		tx, err = wcn.parent.Begin()
	}

	if err != nil {
		return nil, err
	}

	return &wrappedTx{tx, wcn.p, wcn.c}, nil
}

func (wrs *wrappedRows) Columns() []string {
	return wrs.parent.Columns()
}

func (wrs *wrappedRows) Close() (err error) {
	defer func() {
		if err != nil {
			wrs.et.AddError(err)
		}
		wrs.et.Finish()
	}()
	err = wrs.parent.Close()
	return
}

func (wrs *wrappedRows) Next(dest []driver.Value) (err error) {
	panic("not implemented")
}

func (wst *wrappedStmt) Close() (err error) {
	panic("not implemented")
}

func (wst *wrappedStmt) NumInput() int {
	panic("not implemented")
}

func (wst *wrappedStmt) Exec(args []driver.Value) (res driver.Result, err error) {
	panic("not implemented")
}

func (wst *wrappedStmt) Query(args []driver.Value) (r driver.Rows, err error) {
	panic("not implemented")
}

func (wtx *wrappedTx) Commit() (err error) {
	panic("not implemented")
}

func (wtx *wrappedTx) Rollback() (err error) {
	panic("not implemented")
}

func (wrs *wrappedResult) LastInsertId() (id int64, err error) {
	panic("not implemented")
}

func (wrs *wrappedResult) RowsAffected() (n int64, err error) {
	panic("not implemented")
}
