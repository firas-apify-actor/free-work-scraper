package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
)

// charging tracks pay-per-event spend against the user's ACTOR_MAX_TOTAL_CHARGE_USD.
// The platform API does not enforce that limit, so we do.
type charging struct {
	prices map[string]float64 // eventName -> USD; empty = not a pay-per-event run (free)
	max    float64            // +Inf when unset
	spent  float64
	n      int // idempotency counter
	counts map[string]int
	test   bool
}

func (c *Client) loadCharging() *charging {
	ch := &charging{prices: map[string]float64{}, max: math.Inf(1), counts: map[string]int{}}
	if v, err := strconv.ParseFloat(os.Getenv("ACTOR_MAX_TOTAL_CHARGE_USD"), 64); err == nil && v > 0 {
		ch.max = v
	}
	if c.local() {
		if os.Getenv("ACTOR_TEST_PAY_PER_EVENT") == "true" {
			ch.test = true // every event costs $1 locally, like the JS SDK
		}
		return ch
	}
	raw := os.Getenv("ACTOR_PRICING_INFO")
	if raw == "" { // fall back to the run object
		if b, code, err := c.api("GET", "/actor-runs/"+c.runID, nil); err == nil && code < 300 {
			var r struct {
				Data struct{ PricingInfo json.RawMessage }
			}
			if json.Unmarshal(b, &r) == nil {
				raw = string(r.Data.PricingInfo)
			}
		}
	}
	var p struct {
		PricingModel    string `json:"pricingModel"`
		PricingPerEvent struct {
			ActorChargeEvents map[string]struct {
				EventPriceUsd float64 `json:"eventPriceUsd"`
			} `json:"actorChargeEvents"`
		} `json:"pricingPerEvent"`
	}
	if raw != "" && json.Unmarshal([]byte(raw), &p) == nil && p.PricingModel == "PAY_PER_EVENT" {
		for name, e := range p.PricingPerEvent.ActorChargeEvents {
			ch.prices[name] = e.EventPriceUsd
		}
	}
	return ch
}

// Charge bills up to n events and returns how many were charged and whether the
// spending limit now blocks another one. Free runs and unconfigured events are no-ops
// that charge everything (so the Actor can ship free before prices are switched on).
func (c *Client) Charge(event string, n int) (charged int, limitReached bool, err error) {
	if c.charging == nil {
		c.charging = c.loadCharging()
	}
	ch := c.charging
	price, priced := ch.prices[event]
	if ch.test {
		price, priced = 1, true
	}
	if !priced {
		return n, false, nil
	}
	if price > 0 {
		room := int(math.Floor((ch.max-ch.spent)/price + 1e-9))
		if room < 0 {
			room = 0
		}
		charged = min(n, room)
	} else {
		charged = n
	}
	if charged > 0 {
		if c.local() {
			err = c.pushLocal("charging-log", map[string]any{"eventName": event, "count": charged, "priceUsd": price, "totalUsd": price * float64(charged)})
		} else {
			err = c.chargeAPI(event, charged)
		}
		if err != nil {
			return 0, false, err
		}
		ch.spent += price * float64(charged)
		ch.counts[event] += charged
	}
	limitReached = price > 0 && ch.max-ch.spent < price-1e-9
	return charged, limitReached, nil
}

func (c *Client) chargeAPI(event string, n int) error {
	ch := c.charging
	ch.n++
	key := fmt.Sprintf("%s-%s-%d", c.runID, event, ch.n) // same key on retry: no double charge
	body, _ := json.Marshal(map[string]any{"eventName": event, "count": n})
	var lastErr error
	for try := 0; try < 2; try++ {
		out, code, err := c.apiH("POST", "/actor-runs/"+c.runID+"/charge", body, map[string]string{"idempotency-key": key})
		if err == nil && code < 300 {
			return nil
		}
		lastErr = fmt.Errorf("charge %s: %d %v %s", event, code, err, out)
		if err == nil && code < 500 {
			break // client error: retrying won't help
		}
		slog.Warn("charge failed, retrying", "event", event)
	}
	return lastErr
}

// ChargedCounts returns billed events so far (for the end-of-run summary).
func (c *Client) ChargedCounts() map[string]int {
	if c.charging == nil {
		return map[string]int{}
	}
	return c.charging.counts
}
