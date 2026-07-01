# Changelog

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
