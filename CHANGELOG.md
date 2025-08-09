# Changelog

## [1.9.0](https://github.com/staticaland/pin-github-actions/compare/v1.8.0...v1.9.0) (2025-08-09)


### Features

* maybe ([de4d103](https://github.com/staticaland/pin-github-actions/commit/de4d103ed8ee9a0254247c171f0282cc0b648118))
* trigger release ([66b61b6](https://github.com/staticaland/pin-github-actions/commit/66b61b613886238ea4a2daf8f20548fbbb7bcad9))

## [1.8.0](https://github.com/staticaland/pin-github-actions/compare/v1.7.0...v1.8.0) (2025-08-09)


### Features

* Add --yes flag to apply updates non-interactively ([#13](https://github.com/staticaland/pin-github-actions/issues/13)) ([3b437b1](https://github.com/staticaland/pin-github-actions/commit/3b437b106a616e4d7f830d4630e1a637cd6da30d))
* better message ordering ([3515fd2](https://github.com/staticaland/pin-github-actions/commit/3515fd2f98078b9c81a24bb06edbad52aa274d8e))
* output ordering ([69a7ea0](https://github.com/staticaland/pin-github-actions/commit/69a7ea0e1d5d8f7ee597e665f6d00d7b9f747616))
* Track action occurrences by location ([#9](https://github.com/staticaland/pin-github-actions/issues/9)) ([4950f16](https://github.com/staticaland/pin-github-actions/commit/4950f1613ca6efa6b74e7d5180221fc7965207a2))

## [1.7.0](https://github.com/staticaland/pin-github-actions/compare/v1.6.0...v1.7.0) (2025-08-08)


### Features

* Improve tag resolution by collecting and checking candidate tags in two passes ([#10](https://github.com/staticaland/pin-github-actions/issues/10)) ([93c0139](https://github.com/staticaland/pin-github-actions/commit/93c0139cb29a017798e5ece5252fbacdfed03a88))
* Optimize major pinning search heuristics ([#8](https://github.com/staticaland/pin-github-actions/issues/8)) ([0bb5fba](https://github.com/staticaland/pin-github-actions/commit/0bb5fba4bbb0b411e9c7d5c7fcda117ed9200ce7))

## [1.6.0](https://github.com/staticaland/pin-github-actions/compare/v1.5.0...v1.6.0) (2025-08-08)


### Features

* add --expand-major flag to expand whole-number major refs to full semver in comments; still pin to commit SHA\n\n- add flag and wire through resolution\n- map resolved major ref commit to full version tag when possible\n- update README usage/docs ([73bcb22](https://github.com/staticaland/pin-github-actions/commit/73bcb22120533d05af9b5f9fc44a803a29efb385))
* add policy for selecting new versions ([6e84741](https://github.com/staticaland/pin-github-actions/commit/6e8474138a6c7b3487f8eb97821f804b9d8a6ccd))
* fallback to tags for actions without releases; use semver (Masterminds/semver) to pick highest version or newest tag; resolve moving major tags (e.g., v4) to current commit; dereference annotated tags ([73b129c](https://github.com/staticaland/pin-github-actions/commit/73b129ca5247ddb4b04ec1c47c93ce1ef119f761))
* pretty output ([086cdee](https://github.com/staticaland/pin-github-actions/commit/086cdee308b4239866708f8f7f3a5814e2b5861a))


### Bug fixes

* dereference annotated tags to commit SHA when resolving refs ([016e103](https://github.com/staticaland/pin-github-actions/commit/016e1039c9b787c18fba40b93df0fe92dba632c9))

## [1.5.0](https://github.com/staticaland/pin-github-actions/compare/v1.4.0...v1.5.0) (2025-08-07)


### Features

* Use bold in output ([85b3308](https://github.com/staticaland/pin-github-actions/commit/85b330889d21800e8a2a410bf29bc46382d4a125))

## [1.4.0](https://github.com/staticaland/pin-github-actions/compare/v1.3.0...v1.4.0) (2025-08-07)


### Features

* Trigger release ([8dab13a](https://github.com/staticaland/pin-github-actions/commit/8dab13aa8e4e477fdc083aa346aaa5af222db44e))

## [1.3.0](https://github.com/staticaland/pin-github-actions/compare/v1.2.0...v1.3.0) (2025-08-07)


### Features

* Trigger release ([dd416c8](https://github.com/staticaland/pin-github-actions/commit/dd416c8dc3e250ac6dbd2a118e39b6f47f41adda))

## [1.2.0](https://github.com/staticaland/pin-github-actions/compare/v1.1.0...v1.2.0) (2025-08-07)


### Features

* Trigger release ([145811f](https://github.com/staticaland/pin-github-actions/commit/145811f84227e6d7a5f61be6981be9f98f486f8f))

## [1.1.0](https://github.com/staticaland/pin-github-actions/compare/v1.0.0...v1.1.0) (2025-08-07)


### Features

* Trigger release ([29e3c0b](https://github.com/staticaland/pin-github-actions/commit/29e3c0b9564638fa6a263a855e087dfe57f839cf))

## 1.0.0 (2025-08-07)


### Features

* add goreleaser config for homebrew cask and builds ([f1f3326](https://github.com/staticaland/pin-github-actions/commit/f1f3326bfd5379037269e6d70bcbac0ee80c7a0f))
* enhance token authentication with multiple sources ([f7b0fbb](https://github.com/staticaland/pin-github-actions/commit/f7b0fbbd57a1b3eefcc047d7dfc3a5abe416e26a))
* implement GitHub Actions pinning tool in Go ([d466d4b](https://github.com/staticaland/pin-github-actions/commit/d466d4b0532829d38ae63a2b7c4892267eb2bc56))
* initial commit with README ([43c316f](https://github.com/staticaland/pin-github-actions/commit/43c316fa155d3c280838b9ec8feb7b2c798e6722))
