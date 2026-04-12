package mikrotik

import (
	goredis "github.com/redis/go-redis/v9"
	"mikmongo/internal/service"
	svcmikrotik "mikmongo/internal/service/mikrotik"
)

// Registry holds all MikroTik handler instances.
type Registry struct {
	PPP       *PPPHandler
	PPPWS     *PPPWSHandler
	Hotspot   *HotspotHandler
	HotspotWS *HotspotWSHandler
	Queue     *QueueHandler
	Firewall  *FirewallHandler
	IP        *IPHandler
	Monitor   *MonitorHandler
	MonitorWS *MonitorWSHandler
	Raw       *RawHandler
	Cached          *CachedHandler
	Log             *LogHandler
	CollectorStatus *CollectorStatusHandler
}

// NewRegistry creates a new MikroTik handler registry.
func NewRegistry(mkRegistry *svcmikrotik.Registry, routerSvc *service.RouterService, rdb *goredis.Client) *Registry {
	return &Registry{
		PPP:       NewPPPHandler(mkRegistry.PPP),
		PPPWS:     NewPPPWSHandler(routerSvc),
		Hotspot:   NewHotspotHandler(mkRegistry.Hotspot),
		HotspotWS: NewHotspotWSHandler(routerSvc),
		Queue:     NewQueueHandler(mkRegistry.Queue),
		Firewall:  NewFirewallHandler(mkRegistry.Firewall),
		IP:        NewIPHandler(mkRegistry.IPPool, mkRegistry.IPAddress),
		Monitor:   NewMonitorHandler(mkRegistry.Monitor),
		MonitorWS: NewMonitorWSHandler(routerSvc),
		Raw:       NewRawHandler(routerSvc),
		Cached:    NewCachedHandler(rdb),
		Log:       NewLogHandler(rdb),
	}
}
