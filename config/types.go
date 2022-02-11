package config

import (
	"crypto/ecdsa"
	"io"
	"os"
	"time"

	"github.com/realDragonium/Ultraviolet/mc"
)

type ServerConfig struct {
	FilePath string
	Name     string   `json:"name"`
	Domains  []string `json:"domains"`

	ProxyTo           string `json:"proxyTo"`
	ProxyBind         string `json:"proxyBind"`
	DialTimeout       string `json:"dialTimeout"`
	OldRealIP         bool   `json:"useRealIPv2.4"`
	NewRealIP         bool   `json:"useRealIPv2.5"`
	RealIPKey         string `json:"realIPKeyPath"`
	SendProxyProtocol bool   `json:"sendProxyProtocol"`

	DisconnectMessage string `json:"disconnectMessage"`

	CacheStatus         bool            `json:"cacheStatus"`
	CacheUpdateCooldown string          `json:"cacheUpdateCooldown"`
	ValidProtocol       int             `json:"validProtocol"`
	OfflineStatus       mc.SimpleStatus `json:"offlineStatus"`

	RateLimit           int    `json:"rateLimit"`
	RateDuration        string `json:"rateCooldown"`
	RateBanListCooldown string `json:"banListCooldown"`
	RateDisconMsg       string `json:"reconnectMsg"`

	CheckStateOption    string
	StateUpdateCooldown string `json:"stateUpdateCooldown"`
}

type APIServerConfig struct {
	ID string `json:"id"`

	Domains           []string `json:"domains"`
	ProxyTo           string   `json:"proxyTo"`
	ProxyBind         string   `json:"proxyBind"`
	DialTimeout       string   `json:"dialTimeout"`
	SendProxyProtocol bool     `json:"sendProxyProtocol"`

	IsOnline          bool            `json:"isOnline"`
	UseStatusCache    bool            `json:"useStatusCache"`
	CachedStatus      mc.SimpleStatus `json:"cachedStatus"`
	DisconnectMessage string          `json:"disconnectMessage"`
	
	LimitBots bool 
}

func (cfg ServerConfig) ID() string {
	return cfg.FilePath
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ProxyBind:           "",
		DialTimeout:         "1s",
		OldRealIP:           false,
		NewRealIP:           false,
		SendProxyProtocol:   false,
		DisconnectMessage:   "{\"text\": \"Server is offline\"}",
		CacheStatus:         true,
		CacheUpdateCooldown: "1m",
		RateLimit:           5,
		RateDuration:        "1s",
		RateBanListCooldown: "5m",
		RateDisconMsg:       "{\"text\": \"Please reconnect to verify yourself\"}",
		StateUpdateCooldown: "5s",
	}
}

type UltravioletConfig struct {
	ListenTo            string          `json:"listenTo"`
	DefaultStatus       mc.SimpleStatus `json:"defaultStatus"`
	VerifyConnMsg       string          `json:"verifyConnMsg"`
	NumberOfWorkers     int             `json:"numberOfWorkers"`
	NumberOfListeners   int             `json:"numberOfListeners"`
	AcceptProxyProtocol bool            `json:"acceptProxyProtocol"`
	UsePrometheus       bool            `json:"enablePrometheus"`
	PrometheusBind      string          `json:"prometheusBind"`
	APIBind             string          `json:"apiBind"`
	UseTableflip        bool            `json:"useTableflip"`
	PidFile             string          `json:"pidFile"`

	IODeadline time.Duration
	LogOutput  io.Writer
}

func (cfg UltravioletConfig) VerifyConnectionPk() mc.Packet {
	return mc.ClientBoundDisconnect{
		Reason: mc.String(cfg.VerifyConnMsg),
	}.Marshal()
}

func (cfg UltravioletConfig) DefaultStatusPk() mc.Packet {
	return cfg.DefaultStatus.Marshal()
}

func DefaultUltravioletConfig() UltravioletConfig {
	return UltravioletConfig{
		ListenTo: ":25565",
		DefaultStatus: mc.SimpleStatus{
			Name:        "Ultraviolet",
			Protocol:    0,
			Description: "Some proxy didnt proxy",
		},
		VerifyConnMsg:       "{\"text\": \"Please reconnect to verify yourself\"}",
		NumberOfWorkers:     10,
		NumberOfListeners:   1,
		AcceptProxyProtocol: false,
		UsePrometheus:       true,
		PrometheusBind:      ":9100",
		APIBind:             "127.0.0.1:9099",
		PidFile:             "",
		UseTableflip:        true,

		IODeadline: time.Second,
		LogOutput:  os.Stdout,
	}
}

type StateOptions int

const (
	_ StateOptions = iota
	CACHE
	ALWAYS_ONLINE
	ALWAYS_OFFLINE
)

func NewStateOption(option string) StateOptions {
	o := CACHE
	switch option {
	case "online":
		o = ALWAYS_ONLINE
	case "offline":
		o = ALWAYS_OFFLINE
	}
	return o
}

type BackendWorkerConfig struct {
	Name                string
	StateOption         StateOptions
	StateUpdateCooldown time.Duration
	OldRealIp           bool
	NewRealIP           bool
	RealIPKey           *ecdsa.PrivateKey
	CacheStatus         bool
	CacheUpdateCooldown time.Duration
	ValidProtocol       int
	OfflineStatus       mc.Packet
	DisconnectPacket    mc.Packet
	ProxyTo             string
	ProxyBind           string
	DialTimeout         time.Duration
	SendProxyProtocol   bool
	RateLimit           int
	RateLimitStatus     bool
	RateLimitDuration   time.Duration
	RateBanListCooldown time.Duration
	RateDisconPk        mc.Packet
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		IOTimeout: time.Second,
	}
}

type WorkerConfig struct {
	NumberOfWorkers int
	DefaultStatus   mc.SimpleStatus
	IOTimeout       time.Duration
}

func NewWorkerConfig(uvCfg UltravioletConfig) WorkerConfig {
	if uvCfg.IODeadline == 0 {
		uvCfg.IODeadline = time.Second
	}
	return WorkerConfig{
		DefaultStatus: uvCfg.DefaultStatus,
		IOTimeout:     uvCfg.IODeadline,
	}
}

type UVConfigReader = func() (UltravioletConfig, error)

// Will only return the configs if they are deemed usable
// If they contain conflicts of something goes wrong while reading
//  it will return a error
type ServerConfigReader = func() ([]ServerConfig, error)
