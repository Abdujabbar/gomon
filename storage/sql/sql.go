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

type databaseEventTracker interface {
	plugin.EventTracker

	SetQuery(query string)
	SetParams(params []driver.Value)
	SetNamedParams(params []driver.NamedValue)
}

type databaseEventTrackerImpl struct {
	plugin.EventTracker
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
	et     databaseEventTracker
}

type wrappedRows struct {
	parent driver.Rows
	p      *sqlPlugin
	c      *PluginConfig
}

type wrappedStmt struct {
	parent driver.Stmt
	p      *sqlPlugin
	c      *PluginConfig
	et     databaseEventTracker
}

type wrappedTx struct {
	parent driver.Tx
	p      *sqlPlugin
	c      *PluginConfig
}

type wrappedResult struct {
	parent driver.Result
	p      *sqlPlugin
	c      *PluginConfig
}

var _ plugin.Plugin = (*sqlPlugin)(nil)
var _ driver.Driver = (*wrappedDriver)(nil)

var _ driver.Conn = (*wrappedConn)(nil)
var _ driver.ConnBeginTx = (*wrappedConn)(nil)

var _ driver.Rows = (*wrappedRows)(nil)
var _ driver.Stmt = (*wrappedStmt)(nil)
var _ driver.Tx = (*wrappedTx)(nil)
var _ driver.Result = (*wrappedResult)(nil)
var _ databaseEventTracker = (*databaseEventTrackerImpl)(nil)

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

func (s *sqlPlugin) databaseEventTracker() databaseEventTracker {
	tracker := &databaseEventTrackerImpl{plugin.NewEventTracker(s)}
	return tracker
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
			et := wdr.p.databaseEventTracker()
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
	et := wcn.p.databaseEventTracker()
	et.SetFingerprint("conn-prepare")
	et.Set("conn-prepare", query)

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
		et := wcn.p.databaseEventTracker()
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
	tx, err = wcn.parent.(driver.ConnBeginTx).BeginTx(plugin.ContextWith(ctx, et), opts)
	if err != nil {
		et.AddError(err)
		et.Finish()
	} else {
		tx = &wrappedTx{
			parent: tx,
			p:      wcn.p,
			c:      wcn.c,
		}
	}
	return
}

func (wrs *wrappedRows) Columns() []string {
	return wrs.parent.Columns()
}

func (wrs *wrappedRows) Close() (err error) {
	panic("not implemented")
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

func (et *databaseEventTrackerImpl) SetQuery(query string) {
	et.Set(KeyQuery, query)
}

func (et *databaseEventTrackerImpl) SetParams(params []driver.Value) {
	et.Set(KeyParams, params)
}

func (et *databaseEventTrackerImpl) SetNamedParams(params []driver.NamedValue) {
	et.Set(KeyNamedParams, params)
}
