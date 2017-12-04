package runtime

import (
	"context"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/iahmedov/gomon"
)

func init() {
	gomon.SetConfigFunc(pluginName, SetConfig)
}

type PluginConfig struct {
	MemProfileInterval time.Duration
	GSStatInterval     time.Duration
}

type runtimeMetricCollector struct {
	config         PluginConfig
	configReloader chan struct{}

	lastMemStatId uuid.UUID
	lastMemStat   runtime.MemStats
}

var defaultConfig = &PluginConfig{
	MemProfileInterval: time.Second * 5,
	GSStatInterval:     time.Second * 2,
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
		var memProfileTick, gsstatTick = time.Tick(c.config.MemProfileInterval), time.Tick(c.config.GSStatInterval)
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.configReloader:
				memProfileTick = time.Tick(c.config.MemProfileInterval)
				gsstatTick = time.Tick(c.config.GSStatInterval)
			case <-memProfileTick:
				c.collectMemStats()
			case <-gsstatTick:
				// c.collectGCStats()
			}
		}
	}()
}

func (c *runtimeMetricCollector) ReloadConf() {
	c.configReloader <- struct{}{}
}

func (c *runtimeMetricCollector) collectGCStats() {

	et := gomon.FromContext(nil).NewChild(false)
	defer et.Finish()
	var stats debug.GCStats
	debug.ReadGCStats(&stats)

	// fmt.Printf("last collect: %t, %s\n", stats.LastGC.IsZero(), stats.LastGC)
}

func (c *runtimeMetricCollector) collectMemStats() {
	et := gomon.FromContext(nil).NewChild(false)
	defer et.Finish()
	et.SetFingerprint("rt-collect-mem")
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
