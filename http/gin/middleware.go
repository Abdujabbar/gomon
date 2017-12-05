package gin

import (
	"github.com/gin-gonic/gin"
	"github.com/iahmedov/gomon"
	gomonhttp "github.com/iahmedov/gomon/http"
)

func init() {
	gomon.SetConfigFunc(pluginName, SetConfig)
}

var defaultConfig = &gomonhttp.PluginConfig{
	RequestHeaders:  true,
	RespBody:        true,
	RespBodyMaxSize: 1024,
	RespHeaders:     true,
	RespCode:        true,
}

var pluginName = "http-gin"

func SetConfig(c gomon.TrackerConfig) {
	if conf, ok := c.(*gomonhttp.PluginConfig); ok {
		defaultConfig = conf
	} else {
		panic("not compatible config")
	}
}

func (p *PluginConfig) Name() string {
	return pluginName
}

func Monitoring() gin.HandlerFunc {
	return func(c *gin.Context) {
		et := gomonhttp.IncomingRequestTracker(c.Writer, c.Request, defaultConfig)
		et.SetFingerprint("gin-handle")
		defer et.Finish()
		c.Next()
	}
}
