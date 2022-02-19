package exchange

import (
	"context"
	"encoding/json"
	"github.com/rookie-ninja/rk-common/common"
	"github.com/rookie-ninja/rk-entry/entry"
	"go.uber.org/atomic"
	"golang.org/x/text/currency"
	"math"
	"time"
)

// This must be declared in order to register registration function into rk context
// otherwise, rk-boot won't able to bootstrap entry automatically from boot config file
func init() {
	rkentry.RegisterEntryRegFunc(RegisterEntryFromConfig)
}

const (
	EntryName   = "ExchangeEntry"
	EntryType   = "ExchangeEntry"
	Description = "Collect exchange rate information from remote services"
)

// ************** global **************

// Convert global function which convert with exchange rate in ExRateEntry
func Convert(srcUnit, targetUnit string, srcAmount float64) (float64, bool) {
	if currency, ok := GetCurrency(srcUnit, targetUnit); ok {
		return math.Round(srcAmount*currency*100) / 100, true
	}

	return 0.00, false
}

// GetCurrency from dynamic syncer first, the order does not matter
//
// Fall back will be static syncer if set!
func GetCurrency(srcUnit string, targetUnit string) (float64, bool) {
	if v := rkentry.GlobalAppCtx.GetEntry(EntryName); v != nil {
		if entry, ok := v.(*Entry); ok {
			return entry.GetCurrency(srcUnit, targetUnit)
		}
	}

	return 0.00, false
}

// ListCurrency from dynamic syncer first, the order does not matter
//
// Fall back will be static syncer if set!
func ListCurrency(srcUnit string) map[string]float64 {
	if v := rkentry.GlobalAppCtx.GetEntry(EntryName); v != nil {
		if entry, ok := v.(*Entry); ok {
			return entry.ListCurrency(srcUnit)
		}
	}

	return make(map[string]float64)
}

// ************** ExRateEntry **************

// BootConfig bootstrap entry from config
type BootConfig struct {
	Exchange struct {
		Enabled         bool   `yaml:"enabled" json:"enabled"`
		SyncIntervalMin int    `yaml:"syncIntervalMin" json:"syncIntervalMin"`
		BaseUnit        string `yaml:"baseUnit" json:"baseUnit"`
		Static          struct {
			Enabled  bool               `yaml:"enabled" json:"enabled"`
			Currency map[string]float64 `yaml:"currency" json:"currency"`
		} `yaml:"static" json:"static"`
		Provider struct {
			ExchangeRateApi struct {
				Enabled bool   `yaml:"enabled" json:"enabled"`
				ApiKey  string `yaml:"apiKey" json:"apiKey"`
			} `yaml:"exchangeRateApi" json:"exchangeRateApi"`
		} `yaml:"provider" json:"provider"`
	} `yaml:"exchange" json:"exchange"`
}

// RegisterEntryFromConfig create entry from config file
func RegisterEntryFromConfig(configFilePath string) map[string]rkentry.Entry {
	res := make(map[string]rkentry.Entry)

	config := &BootConfig{}

	rkcommon.UnmarshalBootConfig(configFilePath, config)

	if config.Exchange.Enabled {
		// static syncer
		syncers := make([]Syncer, 0)

		if config.Exchange.Static.Enabled {
			static := NewStaticSyncer(
				config.Exchange.BaseUnit,
				config.Exchange.Static.Currency)
			syncers = append(syncers, static)
		}

		// exchange api
		if config.Exchange.Provider.ExchangeRateApi.Enabled {
			exChangeApi := NewExRateApiSyncer(
				config.Exchange.BaseUnit,
				config.Exchange.Provider.ExchangeRateApi.ApiKey)
			syncers = append(syncers, exChangeApi)
		}

		entry := RegisterEntry(
			WithBaseUnit(config.Exchange.BaseUnit),
			WithSyncIntervalMin(config.Exchange.SyncIntervalMin),
			WithSyncer(syncers...))

		res[entry.GetName()] = entry

		rkentry.GlobalAppCtx.AddEntry(entry)
	}

	return res
}

// RegisterEntry register with Option
func RegisterEntry(opts ...Option) *Entry {
	entry := &Entry{
		baseUnit:        currency.USD.String(),
		currency:        newAtomicMapFloat64(),
		syncer:          map[string]Syncer{},
		syncIntervalMin: 60 * 24 * time.Minute,
		shutdownSig:     atomic.NewBool(false),
	}

	for i := range opts {
		opts[i](entry)
	}

	return entry
}

// Entry implementation of rkentry.Entry
type Entry struct {
	baseUnit        string
	currency        *atomicMapFloat64
	syncer          map[string]Syncer
	syncIntervalMin time.Duration
	shutdownSig     *atomic.Bool
}

// Bootstrap entry
func (e *Entry) Bootstrap(ctx context.Context) {
	e.sync()

	if e.currency.Empty() {
		// load static first if exist!
		if v, ok := e.syncer[staticSyncerType]; ok {
			req := &SyncReq{
				BaseUnit: e.baseUnit,
			}
			resp := v.Sync(req)

			if resp.Meta.Success {
				e.currency.Load(resp.Currency)
			}
		}
	}

	go func() {
		// sync currency
		for !e.shutdownSig.Load() {
			e.sync()
			time.Sleep(e.syncIntervalMin)
		}
	}()
}

// Sync from remote server and static value
func (e *Entry) sync() {
	for k, v := range e.syncer {
		if k == staticSyncerType {
			continue
		}

		req := &SyncReq{
			BaseUnit: e.baseUnit,
		}

		resp := v.Sync(req)
		if !resp.Meta.Success {
			continue
		}

		e.currency.Load(resp.Currency)
		break
	}
}

// Interrupt entry
func (e *Entry) Interrupt(ctx context.Context) {
	e.shutdownSig.Store(true)
}

// GetName returns name of entry
func (e *Entry) GetName() string {
	return EntryName
}

// GetType returns type of entry
func (e *Entry) GetType() string {
	return EntryType
}

// GetDescription returns description of entry
func (e *Entry) GetDescription() string {
	return Description
}

// String to string
func (e *Entry) String() string {
	bytes, _ := json.Marshal(e)
	return string(bytes)
}

// MarshalJSON json marshaller
func (e *Entry) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"entryName":        e.GetName(),
		"entryType":        e.GetType(),
		"entryDescription": e.GetDescription(),
	}

	syncers := make([]string, 0)
	for k := range e.syncer {
		syncers = append(syncers, k)
	}
	m["syncers"] = syncers

	return json.Marshal(m)
}

// UnmarshalJSON json unmarshaller
func (e *Entry) UnmarshalJSON([]byte) error {
	return nil
}

// GetCurrency with source unit and target unit
func (e *Entry) GetCurrency(srcUnit, targetUnit string) (float64, bool) {
	currencyMap := e.currency.Copy()

	if e.baseUnit == srcUnit {
		res, ok := currencyMap[targetUnit]
		return res, ok
	} else {
		// need to convert!
		for k, v := range currencyMap {
			if k == srcUnit {
				convertedCurrency := convertCurrencyMap(v, currencyMap)
				res, ok := convertedCurrency[targetUnit]
				return res, ok
			}
		}
	}

	return 0.00, false

}

// ListCurrency list all currency info
func (e *Entry) ListCurrency(srcUnit string) map[string]float64 {
	res := make(map[string]float64)

	currencyMap := e.currency.Copy()

	if e.baseUnit == srcUnit {
		return currencyMap
	} else {
		for k, v := range currencyMap {
			if k == srcUnit {
				return convertCurrencyMap(v, currencyMap)
			}
		}
	}

	return res
}

// *************** Option ***************

// Option entry options
type Option func(e *Entry)

// WithSyncer provide Syncer.
func WithSyncer(in ...Syncer) Option {
	return func(e *Entry) {
		for i := range in {
			if in[i] != nil {
				e.syncer[in[i].GetType()] = in[i]
			}
		}
	}
}

// WithSyncIntervalMin provide intervalMin.
func WithSyncIntervalMin(intervalMin int) Option {
	return func(e *Entry) {
		if intervalMin > 0 {
			e.syncIntervalMin = time.Duration(intervalMin) * time.Minute
		}
	}
}

// WithBaseUnit provide baseUnit.
func WithBaseUnit(baseUnit string) Option {
	return func(e *Entry) {
		if len(baseUnit) > 0 {
			e.baseUnit = baseUnit
		}
	}
}

// Convert currency map with provided base unit
func convertCurrencyMap(srcUnit float64, original map[string]float64) map[string]float64 {
	res := make(map[string]float64)

	for k, v := range original {
		v = 1 / srcUnit * v
		v = math.Round(v*100) / 100
		res[k] = v
	}

	return res
}
