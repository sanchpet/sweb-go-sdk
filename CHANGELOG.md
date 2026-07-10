# Changelog

## [0.9.0](https://github.com/sanchpet/sweb-go-sdk/compare/v0.8.2...v0.9.0) (2026-07-10)


### Features

* add VPS power operations (powerOn/powerOff/reboot, isRunning) ([#25](https://github.com/sanchpet/sweb-go-sdk/issues/25)) ([584f061](https://github.com/sanchpet/sweb-go-sdk/commit/584f061747cde2dcdc32d6e437ddf42228ae5c72))

## [0.8.2](https://github.com/sanchpet/sweb-go-sdk/compare/v0.8.1...v0.8.2) (2026-07-02)


### Bug Fixes

* decode local_ip/ips as array OR bare object ([#21](https://github.com/sanchpet/sweb-go-sdk/issues/21)) ([f6ef0bd](https://github.com/sanchpet/sweb-go-sdk/commit/f6ef0bd1bed20df6dccb90a0d56ef994c7068db9))

## [0.8.1](https://github.com/sanchpet/sweb-go-sdk/compare/v0.8.0...v0.8.1) (2026-07-02)


### Bug Fixes

* decode IP price as FlexFloat (API returns fractional money) ([#19](https://github.com/sanchpet/sweb-go-sdk/issues/19)) ([220d5ac](https://github.com/sanchpet/sweb-go-sdk/commit/220d5ac8a5f50c5de472a959cd98f1bb4d20b46d))

## [0.8.0](https://github.com/sanchpet/sweb-go-sdk/compare/v0.7.0...v0.8.0) (2026-07-02)


### Features

* add IP service with local-network attach/detach ([#17](https://github.com/sanchpet/sweb-go-sdk/issues/17)) ([8c42712](https://github.com/sanchpet/sweb-go-sdk/commit/8c42712fe18e1aa9ccdc6c80e3f29b5d50b76380))

## Changelog

From v0.8.0 on, this file is maintained automatically by
[release-please](https://github.com/googleapis/release-please) from
[Conventional Commit](https://www.conventionalcommits.org/) messages — see
[CONTRIBUTING.md](CONTRIBUTING.md).

## Releases up to 0.7.0 (pre-automation)

- **0.7.0** — `VPS.WaitForIdle`: poll until `current_action` settles (through the async `Modify → ExtIpAdd → …` sequence; `is_running` stays 1).
- **0.6.1** — fix: `changePlan` sends the `planId` parameter (the docs' example wrongly showed `vpsPlanId`; fixes `-32602`).
- **0.6.0** — guard the configurator against resolving to a sold-out plan (clear error instead of a cryptic create failure).
- **0.5.0** — `VPS.ChangePlan`: in-place tariff change (resize without reprovisioning).
- **0.4.0** — `VPS.GetFirstOrderInfo` (unwraps the nested JSON-RPC envelope).
- **0.3.0** — reconcile the `VPS` (index) struct with the real API; `FlexInt`/`FlexFloat` decode fields returned as number-or-quoted-string; fixes `List` crashing on a quoted `ram`/`plan_id`.
- **0.2.1** — decode money fields (`plan_price`, `price_per_month`) as `float64` (the API returns fractional prices).
- **0.2.0** — `VPS.Rename` (in-place alias change).
- **0.1.4** — `VPS.GetConstructorPlanID`.
- **0.1.3** — transparent token refresh (`WithCredentials`).
- **0.1.2** — `VPS.Remove`.
- **0.1.0** — initial: JSON-RPC transport, `CreateToken`, `VPS.List`/`Create`/`AvailableConfig`.
