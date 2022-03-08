package rkex

import (
	"errors"
	"fmt"
	"golang.org/x/text/currency"
	"math"
	"strings"
)

const staticSyncerType = "StaticSyncer"

// NewStaticSyncer create new StaticSyncer
func NewStaticSyncer(base string, cur map[string]float64) *StaticSyncer {
	res := &StaticSyncer{
		BaseUnit: currency.USD.String(),
		Currency: make(map[string]float64),
	}

	base = strings.ToUpper(base)

	if len(base) > 0 {
		res.BaseUnit = base
	}

	if cur != nil {
		for k, v := range cur {
			res.Currency[strings.ToUpper(k)] = v
		}
	}

	res.Currency[base] = 1

	return res
}

// StaticSyncer stores static currency info defined by user
type StaticSyncer struct {
	BaseUnit string
	Currency map[string]float64
}

// GetType returns type of Syncer
func (s *StaticSyncer) GetType() string {
	return staticSyncerType
}

// Sync fetch data from local memory
func (s *StaticSyncer) Sync(req *SyncReq) *SyncResp {
	resp := NewSyncResp(req)

	if s.BaseUnit == req.BaseUnit {
		// case 1: equals to base
		resp.Meta.SourceType = s.GetType()
		resp.Meta.Success = true

		// add currency
		for k, v := range s.Currency {
			resp.Currency[k] = math.Round(v*100) / 100
		}
	} else {
		// case 2: convert it!
		for k, v := range s.Currency {
			if req.BaseUnit == k {
				resp.Currency = convertCurrencyMap(v, s.Currency)
			}
		}
	}

	if len(resp.Currency) < 1 {
		resp.Meta.Error = errors.New(fmt.Sprintf("base unit not found, baseUnit:%s", req.BaseUnit))
	}

	return resp
}
