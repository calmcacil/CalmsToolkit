# Changelog

## 1.0.0 (2026-07-15)


### ⚠ BREAKING CHANGES

* unify CalmsToolkit CLI and delivery pipeline ([#37](https://github.com/calmcacil/CalmsToolkit/issues/37))

### Features

* add arr-feed ([2f89a0b](https://github.com/calmcacil/CalmsToolkit/commit/2f89a0be0bbd8f051fb34f56137a9aa65111c14c)), closes [#9](https://github.com/calmcacil/CalmsToolkit/issues/9)
* add media-airtime command for fuzzy Sonarr/Radarr airtime lookup ([4062a5f](https://github.com/calmcacil/CalmsToolkit/commit/4062a5f63bb78b1783c0eb5f5c41f09649ee8bf9))
* add media-airtime command for fuzzy Sonarr/Radarr airtime lookup ([46d0a01](https://github.com/calmcacil/CalmsToolkit/commit/46d0a0120125bfe76f0c6cd8b58b4f4534fcaaf1))
* Add media-calendar table for checking current day and future days calendar entries in cli. ([1a7e05b](https://github.com/calmcacil/CalmsToolkit/commit/1a7e05bb10cca811f460a0bf93fde8b330ae97b8))
* resolve issues [#20](https://github.com/calmcacil/CalmsToolkit/issues/20), [#21](https://github.com/calmcacil/CalmsToolkit/issues/21), [#22](https://github.com/calmcacil/CalmsToolkit/issues/22), [#23](https://github.com/calmcacil/CalmsToolkit/issues/23), [#24](https://github.com/calmcacil/CalmsToolkit/issues/24) ([7de72b0](https://github.com/calmcacil/CalmsToolkit/commit/7de72b0d1d6da440791c1fdd0ad82fc2076a3161))
* unify CalmsToolkit CLI and delivery pipeline ([#37](https://github.com/calmcacil/CalmsToolkit/issues/37)) ([0f819cc](https://github.com/calmcacil/CalmsToolkit/commit/0f819ccfd4507bcb3c74289c71997b6b8e14582d))
* unify config across all tools, add JSON config and make setup wizard ([83489d9](https://github.com/calmcacil/CalmsToolkit/commit/83489d9c3933a691fcda985286520b35799cb44f))
* unify config across all tools, add JSON config file and make setup wizard ([7b34851](https://github.com/calmcacil/CalmsToolkit/commit/7b34851128064c65d97e2850a01b66a97799d978))


### Bug Fixes

* **arr-feed:** address PR feedback - custom formats, sorting, and table layout ([1949246](https://github.com/calmcacil/CalmsToolkit/commit/1949246ad44b0687086cc8df2b460043dbb45ccb))
* **arr-feed:** correct sorting order and remove invalid API parameter ([8aa067b](https://github.com/calmcacil/CalmsToolkit/commit/8aa067b45d57a6552f42ef2b892be0ae74856b71))
* **arr-feed:** decode wrapped history; add -events ([57561a5](https://github.com/calmcacil/CalmsToolkit/commit/57561a57511580999bde5d6648fdd37906fd11e0))
* convert airtime to local timezone and rewrite card rendering ([8e713b6](https://github.com/calmcacil/CalmsToolkit/commit/8e713b6d3ebaaef5e6104b50b6ad7c16e5ee2ba5))
* convert UTC times to local timezone across calendar, feed, and requests ([905e7dd](https://github.com/calmcacil/CalmsToolkit/commit/905e7dda7978e2e678c8ad5c832abf7139a1d10c))
* correct card rendering width and add -full-season flag ([ffb0f84](https://github.com/calmcacil/CalmsToolkit/commit/ffb0f84e9f09f5607df028a77859f10904225a3a))
* handle Ctrl+C in interactive selection prompt ([26c7275](https://github.com/calmcacil/CalmsToolkit/commit/26c72757d4c07cafd0a20d445267d3c6cd056780))
* resolve issues [#20](https://github.com/calmcacil/CalmsToolkit/issues/20), [#21](https://github.com/calmcacil/CalmsToolkit/issues/21), [#22](https://github.com/calmcacil/CalmsToolkit/issues/22), [#23](https://github.com/calmcacil/CalmsToolkit/issues/23), [#24](https://github.com/calmcacil/CalmsToolkit/issues/24) ([3f63257](https://github.com/calmcacil/CalmsToolkit/commit/3f63257ce943062fedb0eab2fcaae49c129e855f))
* rewrite interactive selection prompt to match card-style layout ([49101b0](https://github.com/calmcacil/CalmsToolkit/commit/49101b09ae58a5392a4ae572949c98d388a6042e))
* use calendar-day arithmetic in formatRelativeDate ([4e74fd0](https://github.com/calmcacil/CalmsToolkit/commit/4e74fd07037e1a47b7d30afcc41db50e316bcc68))

## Changelog

All notable public changes to CalmsToolkit will be documented here.

The first public release will be `v1.0.0`. Release Please maintains this file
from Conventional Commit titles after changes merge to `main`.
