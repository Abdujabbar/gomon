package runtime

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iahmedov/gomon"
)

func init() {
	gomon.SetConfigFunc(pluginName, SetConfig)
}

type MemProfileOrder int

type PluginConfig struct {
	MemStatInterval    time.Duration
	MemProfileInterval time.Duration
	MemProfileLimit    int
	MemProfileOrderBy  MemProfileOrder
	MemProfileOrderAsc bool
}

type runtimeMetricCollector struct {
	config         PluginConfig
	configReloader chan struct{}

	lastMemStatId uuid.UUID
	lastMemStat   runtime.MemStats
}

type memProfileSorter struct {
	items     []runtime.MemProfileRecord
	order     MemProfileOrder
	ascending bool
}

type memProfileData struct {
	allocBytes, freeBytes     int64 // number of bytes allocated, freed
	allocObjects, freeObjects int64 // number of objects allocated, freed
	stack                     []string
}

const (
	MemProfileOrderMemAllocation MemProfileOrder = iota
	MemProfileOrderMemFree
	MemProfileOrderAllocObjCount
	MemProfileOrderFreeObjCount
	MemProfileOrderActiveMem
	MemProfileOrderActiveObj
)

var defaultConfig = &PluginConfig{
	MemStatInterval:    time.Second * 5,
	MemProfileInterval: time.Second * 5,
	MemProfileLimit:    5,
	MemProfileOrderBy:  MemProfileOrderMemAllocation,
	MemProfileOrderAsc: false,
}

var runtimeCollector = &runtimeMetricCollector{
	config:         *defaultConfig,
	configReloader: make(chan struct{}, 1),
}
var pluginName = "runtime"

func SetConfig(c gomon.TrackerConfig) {
	if conf, ok := c.(*PluginConfig); ok {
		defaultConfig = conf
		runtimeCollector.config = *conf
		runtimeCollector.ReloadConf()
	} else {
		panic("not compatible config")
	}
}

func (p *PluginConfig) Name() string {
	return pluginName
}

func (c *runtimeMetricCollector) Run(ctx context.Context) {
	c.collectBaseInformation()
	go func() {
		var memStatTick = time.Tick(c.config.MemStatInterval)
		var memProfileTick = time.Tick(c.config.MemProfileInterval)
		var memProfileOrder = c.config.MemProfileOrderBy
		var memProfileAsc = c.config.MemProfileOrderAsc
		var memProfileLimit = c.config.MemProfileLimit
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.configReloader:
				memProfileTick = time.Tick(c.config.MemStatInterval)
				memProfileTick = time.Tick(c.config.MemProfileInterval)
				memProfileOrder = c.config.MemProfileOrderBy
				memProfileAsc = c.config.MemProfileOrderAsc
				memProfileLimit = c.config.MemProfileLimit
			case <-memStatTick:
				c.collectMemStats()
			case <-memProfileTick:
				c.collectMemProfile(memProfileOrder, memProfileLimit, memProfileAsc)
			}
		}
	}()
}

func (c *runtimeMetricCollector) ReloadConf() {
	select {
	default:
	case c.configReloader <- struct{}{}:
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *runtimeMetricCollector) collectMemProfile(order MemProfileOrder, limit int, asc bool) {
	et := gomon.FromContext(nil).NewChild(false)
	et.SetFingerprint("rt-collect-memprof")
	defer et.Finish()
	n, _ := runtime.MemProfile(nil, true)
	if n == 0 {
		return
	}

	p := make([]runtime.MemProfileRecord, n)
	n, ok := runtime.MemProfile(p, true)
	if !ok {
		return
	}

	sort.Sort(memProfileSorter{p, order, asc})
	p = p[:min(len(p), limit)]

	funcCache := make(map[uintptr]string)
	_ = funcCache

	memData := make([]*memProfileData, 0, len(p))
	for _, rec := range p {
		stack := make([]string, 0, 5)
		for _, funcPc := range rec.Stack0 {
			if funcPc == 0 {
				break
			}

			if s, ok := funcCache[funcPc]; ok {
				stack = append(stack, s)
			} else {
				fnc := runtime.FuncForPC(funcPc)
				fl, ln := fnc.FileLine(funcPc)
				s = fmt.Sprintf("[%s]:[%d] [%s]", fl, ln, fnc.Name())
				funcCache[funcPc] = s
				stack = append(stack, s)
			}
		}

		memData = append(memData, &memProfileData{
			allocBytes:   rec.AllocBytes,
			allocObjects: rec.AllocObjects,
			freeBytes:    rec.FreeBytes,
			freeObjects:  rec.FreeObjects,
			stack:        stack,
		})
	}

	topRecords := make([]map[string]interface{}, 0, len(memData))
	for _, m := range memData {
		topRecords = append(topRecords, m.KVData())
	}
	et.Set("mem-profile", topRecords)
}

func (c *runtimeMetricCollector) collectMemStats() {
	et := gomon.FromContext(nil).NewChild(false)
	defer et.Finish()
	et.SetFingerprint("rt-collect-memstat")
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fillMemStat(&m, et)

	// shows memory use between mem profiles,
	// spikes in this value means spike in application
	// memory use, if it correlates with Frees, then ok,
	// if not, maybe we have some memory leaks
	et.Set("diff-total-alloc", m.TotalAlloc-c.lastMemStat.TotalAlloc)
	et.Set("diff-mallocs", m.Mallocs-c.lastMemStat.Mallocs)
	et.Set("diff-frees", m.Frees-c.lastMemStat.Frees)
	et.Set("diff-heap-alloc", m.HeapAlloc-c.lastMemStat.HeapAlloc)
	et.Set("diff-heap-objects", m.HeapObjects-c.lastMemStat.HeapObjects)

	// if this value becomes bigger overtime, maybe we should
	// disable profiling, since its related to runtime memory
	et.Set("diff-mspan-inuse", m.MSpanInuse-c.lastMemStat.MSpanInuse)
	et.Set("prev-tracker-id", c.lastMemStatId)

	c.lastMemStatId = et.ID()
	c.lastMemStat = m
}

func fillMemStat(m *runtime.MemStats, et gomon.EventTracker) {
	et.Set("alloc", m.Alloc)
	et.Set("total-alloc", m.TotalAlloc)
	et.Set("sys", m.Sys)
	et.Set("lookups", m.Lookups)
	et.Set("mallocs", m.Mallocs)
	et.Set("frees", m.Frees)
	et.Set("live-obj", m.Mallocs-m.Frees)
	et.Set("heap-alloc", m.HeapAlloc) // same as m.Alloc
	et.Set("heap-sys", m.HeapSys)
	et.Set("heap-idle", m.HeapIdle)
	et.Set("heap-inuse", m.HeapInuse)
	et.Set("heap-objects", m.HeapObjects)
	et.Set("stack-inuse", m.StackInuse)
	et.Set("stack-sys", m.StackSys)
	et.Set("mspan-inuse", m.MSpanInuse)
	et.Set("mspan-sys", m.MSpanSys)
	et.Set("mcache-inuse", m.MCacheInuse)
	et.Set("mcache-sys", m.MCacheSys)
	et.Set("buck-hashsys", m.BuckHashSys)
	et.Set("gc-sys", m.GCSys)
	et.Set("other-sys", m.OtherSys)
	et.Set("next-gc", m.NextGC)
	et.Set("last-gc", time.Duration(m.LastGC)/time.Nanosecond)
	et.Set("pause-totalns", m.PauseTotalNs)
	et.Set("pause-ns", m.PauseNs[(m.NumGC+255)%256])
	et.Set("pause-pauseend", m.PauseEnd[(m.NumGC+255)%256])
	et.Set("num-gc", m.NumGC)
	et.Set("num-forcegc", m.NumForcedGC)
	et.Set("gc-cpu-fraction", m.GCCPUFraction)
	// et.Set("enable-gc", m.EnableGC) // always true
	// et.Set("debug-gc", m.DebugGC) // currently not used
}

func (c *runtimeMetricCollector) collectBaseInformation() {
	et := gomon.FromContext(nil).NewChild(false)
	et.SetFingerprint("runtime-base")

	et.Set("num-cpu", runtime.NumCPU())
	et.Set("mem-profile-rate", runtime.MemProfileRate)
	et.Set("max-procs", runtime.GOMAXPROCS(0))
	et.Set("go-version", runtime.Version())
	et.Finish()
}

func Run(ctx context.Context) {
	runtimeCollector.Run(ctx)
}

func opAsc(a, b int64) bool {
	return a < b
}

func opDesc(a, b int64) bool {
	return a > b
}

func (m memProfileSorter) Len() int      { return len(m.items) }
func (m memProfileSorter) Swap(i, j int) { m.items[i], m.items[j] = m.items[j], m.items[i] }
func (m memProfileSorter) Less(i, j int) bool {
	var op func(a, b int64) bool
	if m.ascending {
		op = opAsc
	} else {
		op = opDesc
	}
	switch m.order {
	case MemProfileOrderMemAllocation:
		return op(m.items[i].AllocBytes, m.items[j].AllocBytes)
	case MemProfileOrderMemFree:
		return op(m.items[i].FreeBytes, m.items[j].FreeBytes)
	case MemProfileOrderAllocObjCount:
		return op(m.items[i].AllocObjects, m.items[j].AllocObjects)
	case MemProfileOrderFreeObjCount:
		return op(m.items[i].FreeObjects, m.items[j].FreeObjects)
	case MemProfileOrderActiveMem:
		return op(m.items[i].InUseBytes(), m.items[j].InUseBytes())
	case MemProfileOrderActiveObj:
		return op(m.items[i].InUseObjects(), m.items[j].InUseObjects())
	}

	return op(m.items[i].AllocBytes, m.items[j].AllocBytes)
}

func (m *memProfileData) KVData() map[string]interface{} {
	mp := make(map[string]interface{})

	mp["alloc_bytes"] = m.allocBytes
	mp["free_bytes"] = m.freeBytes
	mp["alloc_obj"] = m.allocObjects
	mp["free_obj"] = m.freeObjects
	mp["inuse_bytes"] = m.allocBytes - m.freeBytes
	mp["inuse_obj"] = m.allocObjects - m.freeObjects
	mp["stack"] = strings.Join(m.stack, "\n")

	return mp
}

func (m *memProfileData) String() string {
	return fmt.Sprintf("[%d, %d, %d, %d]\n%s\n\n", m.allocBytes, m.freeBytes, m.allocObjects, m.freeObjects, strings.Join(m.stack, "\n"))
}
