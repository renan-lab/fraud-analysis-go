# 0008. Add optional transaction geolocation

- Status: accepted
- Date: 2026-11-02

## Context

The planned analysis engine needs location to detect impossible travel and purchases far from the cardholder's usual area. The contract only carried `merchant.country`, which is too coarse for a speed check (km/h between two purchases of the same card) and says nothing useful for online purchases, where the merchant can be anywhere.

## Decision

`POST /transactions` accepts an optional `location` object `{"lat": <number>, "lon": <number>}` in WGS 84 decimal degrees, meaning *where the purchase happened*: the terminal for `card_present` and `contactless`, the device for `ecommerce`.

- Absent or `null` means unknown. Once the object is sent, `lat` ([-90, 90]) and `lon` ([-180, 180]) are both required; strings are decode errors, out-of-range values are validation errors.
- `events.Transaction.Location` is a pointer with `omitempty`: the event omits the key when the client sent none, so consumers never confuse "unknown" with the point `0,0`.
- The change is additive and optional, so `TransactionReceivedSchemaVersion` stays `1` (per ADR 0005, only breaking changes bump it).

## Alternatives considered

- Two blocks, `merchant.location` and `device_location`: more precise, but the engine would have to pick one per channel on every rule; one block whose meaning depends on the channel keeps rules simple.
- Merchant location only: useless for `ecommerce`, the channel where most card-not-present fraud happens.
- Required field: simpler for the engine, but real clients often lack a GPS fix or permission, and a required field would be a breaking change to the event.
- Server-side IP geolocation: no client trust needed, but needs a GeoIP database and is city-level at best; can be added later as enrichment in `analysis`.

## Consequences

- Rules must handle a missing location (fall back to `merchant.country` or skip the rule).
- Clients control the coordinates; the engine should treat them as a signal, not proof.
- Making `location` required or adding precision metadata later is a breaking change and needs a new schema version.
