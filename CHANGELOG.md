# Changelog

## [0.15.0](https://github.com/sanchpet/sweb-go-sdk/compare/v0.14.1...v0.15.0) (2026-07-13)


### ⚠ BREAKING CHANGES

* import paths and standalone type references change (sweb.FlexInt -> flex.Int, sweb.BalancerConfig -> balancer.Config, the transport moves under internal/, etc.). Call sites and facade fields (client.VPS.*, sweb.New, sweb.WithToken) are preserved.

### Features

* **backup:** add updateIndex ([#48](https://github.com/sanchpet/sweb-go-sdk/issues/48)) ([aba558b](https://github.com/sanchpet/sweb-go-sdk/commit/aba558bfb1806adf9adad461f8b382aa4b5edb17))
* **balancer:** add load-balancer service ([#57](https://github.com/sanchpet/sweb-go-sdk/issues/57)) ([a78c8cb](https://github.com/sanchpet/sweb-go-sdk/commit/a78c8cbc0e9f3c74a28c1a426c03fa941be3e727)), closes [#53](https://github.com/sanchpet/sweb-go-sdk/issues/53)
* **dbaas:** add managed-database service ([#58](https://github.com/sanchpet/sweb-go-sdk/issues/58)) ([fb70d00](https://github.com/sanchpet/sweb-go-sdk/commit/fb70d001762939336c0fa92f092640a0f65fd56a))
* **ip:** complete VPS IP protected-IP methods ([#51](https://github.com/sanchpet/sweb-go-sdk/issues/51)) ([887d5dc](https://github.com/sanchpet/sweb-go-sdk/commit/887d5dca0340b6b4be002c185dfeb7cdbea1aa85))
* **monitoring:** add monitoring, checks, and contacts services ([#60](https://github.com/sanchpet/sweb-go-sdk/issues/60)) ([8aa6cf6](https://github.com/sanchpet/sweb-go-sdk/commit/8aa6cf616db8b14b5597aaaa65dadbb9efbe6474))
* **ssl:** add VPS SSL-certificate service ([#59](https://github.com/sanchpet/sweb-go-sdk/issues/59)) ([0fab162](https://github.com/sanchpet/sweb-go-sdk/commit/0fab162121c1dedcb5af81236bfdd342d064f253))
* **vps:** complete VPS core methods ([#50](https://github.com/sanchpet/sweb-go-sdk/issues/50)) ([9016673](https://github.com/sanchpet/sweb-go-sdk/commit/9016673431df6ce6e96025f134561ff22b700954))


### Code Refactoring

* per-service packages (ADR-0019) ([#61](https://github.com/sanchpet/sweb-go-sdk/issues/61)) ([cc1137c](https://github.com/sanchpet/sweb-go-sdk/commit/cc1137c5fae00a032097b0a32b5d6b45e486ab9a))

## [0.14.1](https://github.com/sanchpet/sweb-go-sdk/compare/v0.14.0...v0.14.1) (2026-07-12)


### Bug Fixes

* DNS record removal uses the wire verb "del" with a type discriminator ([#40](https://github.com/sanchpet/sweb-go-sdk/issues/40)) ([51ca376](https://github.com/sanchpet/sweb-go-sdk/commit/51ca3763a5ffe337ddad2c8ed04d1988ddeb0fd5))

## [0.14.0](https://github.com/sanchpet/sweb-go-sdk/compare/v0.13.1...v0.14.0) (2026-07-12)


### Features

* add Domains service for the /domains lifecycle ([#38](https://github.com/sanchpet/sweb-go-sdk/issues/38)) ([f70ec3f](https://github.com/sanchpet/sweb-go-sdk/commit/f70ec3fe57038867fed0b3871321b6a79876394c))

## [0.13.1](https://github.com/sanchpet/sweb-go-sdk/compare/v0.13.0...v0.13.1) (2026-07-11)


### Bug Fixes

* decode DNS info result returned as a bare array ([#36](https://github.com/sanchpet/sweb-go-sdk/issues/36)) ([980dc2b](https://github.com/sanchpet/sweb-go-sdk/commit/980dc2b5acce97c6d4d94acecc2bf5a89c5c00b1))

## [0.13.0](https://github.com/sanchpet/sweb-go-sdk/compare/v0.12.0...v0.13.0) (2026-07-11)


### Features

* add DNS-zone service (/domains/dns) ([#34](https://github.com/sanchpet/sweb-go-sdk/issues/34)) ([7ea5a0e](https://github.com/sanchpet/sweb-go-sdk/commit/7ea5a0e9b7c679523cce866a27c0f0a556ed836e))

## [0.12.0](https://github.com/sanchpet/sweb-go-sdk/compare/v0.11.0...v0.12.0) (2026-07-10)


### Features

* add local and cloud backup services (/vps/backup, /vps/remoteBackup) ([#32](https://github.com/sanchpet/sweb-go-sdk/issues/32)) ([0dc48cd](https://github.com/sanchpet/sweb-go-sdk/commit/0dc48cdac0f91993ed439ed7e4b8d7c2210ee156))

## [0.11.0](https://github.com/sanchpet/sweb-go-sdk/compare/v0.10.0...v0.11.0) (2026-07-10)


### Features

* add IP add/remove/move and PTR get/edit (/vps/ip) ([#30](https://github.com/sanchpet/sweb-go-sdk/issues/30)) ([e891b21](https://github.com/sanchpet/sweb-go-sdk/commit/e891b217d1022a6d34013ec5f5e70603026528a0))

## [0.10.0](https://github.com/sanchpet/sweb-go-sdk/compare/v0.9.0...v0.10.0) (2026-07-10)


### Features

* add VPS reinstall/clone/logs (reinstallOs, copy, logs) ([#28](https://github.com/sanchpet/sweb-go-sdk/issues/28)) ([732ad49](https://github.com/sanchpet/sweb-go-sdk/commit/732ad49c8142ef9809b6696c57f45e75933a6644))

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
