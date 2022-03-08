package rkex

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
)

const (
	exRateApiSyncerType = "ExRateApiSyncer"
	exRateApiUrl        = "https://v6.exchangerate-api.com/v6/%s/latest/%s"
)

// NewExRateApiSyncer create new ExRateApiSyncer
func NewExRateApiSyncer(base, token string) *ExRateApiSyncer {
	base = strings.ToUpper(base)

	res := &ExRateApiSyncer{
		BaseUnit: base,
		Token:    token,
	}

	return res
}

// ExRateApiSyncer fetch currency info from remote server
type ExRateApiSyncer struct {
	BaseUnit string
	Token    string
}

// GetType returns type of Syncer
func (e *ExRateApiSyncer) GetType() string {
	return exRateApiSyncerType
}

// Sync fetch data from remote server
func (e *ExRateApiSyncer) Sync(req *SyncReq) *SyncResp {
	resp := NewSyncResp(req)

	rawResp, err := http.Get(fmt.Sprintf(exRateApiUrl, e.Token, e.BaseUnit))

	if err != nil {
		resp.Meta.Error = err
		return resp
	}

	bytes, err := io.ReadAll(rawResp.Body)
	if err != nil {
		resp.Meta.Error = err
		return resp
	}

	innerType := struct {
		Result          string             `json:"result"`
		BaseCode        string             `json:"base_code"`
		ConversionRates map[string]float64 `json:"conversion_rates"`
	}{}

	err = json.Unmarshal(bytes, &innerType)
	if err != nil {
		resp.Meta.Error = err
		return resp
	}

	for k, v := range innerType.ConversionRates {
		resp.Currency[k] = math.Round(v*100) / 100
	}
	resp.Meta.Success = true

	return resp
}
