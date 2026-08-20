# CHANGELOG

<!-- version list -->

## v1.8.0 (2026-08-20)

### Chores

- **agents**: Add skill definitions for agent orchestration
  ([`12e6425`](https://github.com/igorrochap/commitgen/commit/12e6425b9a84f3b790e63ee48813cc225ca9a5c8))

### Features

- **cli**: Add push command to upload commits ([#9](https://github.com/igorrochap/commitgen/pull/9),
  [`b07feb9`](https://github.com/igorrochap/commitgen/commit/b07feb97c7a9e2d29b18db50a6d772a3dfafc875))


## v1.7.0 (2026-07-09)

### Features

- **core**: Add multi-LLM provider support and loading states
  ([#7](https://github.com/igorrochap/commitgen/pull/7),
  [`282ba6b`](https://github.com/igorrochap/commitgen/commit/282ba6bfcb0c75e497068cf9deb7c4d5d25bf17a))

- **core**: Add support for multiple LLM providers
  ([#7](https://github.com/igorrochap/commitgen/pull/7),
  [`282ba6b`](https://github.com/igorrochap/commitgen/commit/282ba6bfcb0c75e497068cf9deb7c4d5d25bf17a))

### Testing

- **generator**: Add loading state integration tests
  ([#7](https://github.com/igorrochap/commitgen/pull/7),
  [`282ba6b`](https://github.com/igorrochap/commitgen/commit/282ba6bfcb0c75e497068cf9deb7c4d5d25bf17a))


## v1.6.0 (2026-06-12)

### Features

- **core**: Add context flag for generation ([#6](https://github.com/igorrochap/commitgen/pull/6),
  [`d131f6b`](https://github.com/igorrochap/commitgen/commit/d131f6bce5b639e1c3fdfe785e7a0fd67ab5d906))


## v1.5.0 (2026-05-28)

### Features

- **pr**: Add command to generate Pull Request descriptions
  ([#5](https://github.com/igorrochap/commitgen/pull/5),
  [`8e982b0`](https://github.com/igorrochap/commitgen/commit/8e982b02a236cb20d5d60bb7ee670acce9ac6f22))


## v1.4.0 (2026-05-05)

### Chores

- **config**: Update default model to gemma4:31b-cloud
  ([`c97a665`](https://github.com/igorrochap/commitgen/commit/c97a6655a4771c0e3b6d817d31143d26a2a45adc))

### Features

- **cli**: Add version command and flag
  ([`d3f0ee6`](https://github.com/igorrochap/commitgen/commit/d3f0ee63be6ebe015c44cffb1116165dde76fab6))


## v1.3.0 (2026-05-05)

### Features

- **core**: Add automatic update notifications
  ([`bf8e6c7`](https://github.com/igorrochap/commitgen/commit/bf8e6c7f774714eb3158f50f686dd16f2eeec968))


## v1.2.0 (2026-05-05)

### Features

- **config**: Add persistent configuration management
  ([`a09bf2a`](https://github.com/igorrochap/commitgen/commit/a09bf2a2c98d29dbe01601f4dec0474004d0709d))


## v1.1.0 (2026-04-29)

### Chores

- **cli**: Update default Ollama model
  ([`0882f71`](https://github.com/igorrochap/commitgen/commit/0882f71841b00d3ab24b739e18cf1afcafa332d0))

### Features

- **git**: Stop automatically staging changes
  ([`87edb28`](https://github.com/igorrochap/commitgen/commit/87edb28fcc491ca6e297389ca098cc6f390f6d00))


## v1.0.1 (2026-04-29)

### Bug Fixes

- **generator**: Replace ollama CLI with HTTP API to handle diffs with forward slashes
  ([`9078b50`](https://github.com/igorrochap/commitgen/commit/9078b509c0118cb44c0fa8be9a7f55c823f60c71))


## v1.0.0 (2026-04-29)

### Bug Fixes

- **ci**: Correct semantic release tag format
  ([`4cc4f81`](https://github.com/igorrochap/commitgen/commit/4cc4f81f2214be8226b2b38efc5de0b7942e41fc))

- **config**: Fix project name quotes
  ([`1ef8215`](https://github.com/igorrochap/commitgen/commit/1ef82152dbd845299395f688473aed2d008f0483))

- **config**: Resolve TOML syntax errors
  ([`9b1b4e5`](https://github.com/igorrochap/commitgen/commit/9b1b4e587b4e6684de33b59264a2408f7b582d94))

### Chores

- **ci**: Set up automated semantic release
  ([`07c904f`](https://github.com/igorrochap/commitgen/commit/07c904fbd6c022f5a8edc3fb52b60898d6ea6725))

- **config**: Fix build_command formatting
  ([`3c74503`](https://github.com/igorrochap/commitgen/commit/3c7450339acb5970f93451f3650c40e786b06eb5))

- **release**: Fix semantic-release tag format
  ([`7c7370e`](https://github.com/igorrochap/commitgen/commit/7c7370ec61a7e099d76d987986af6099d9df2620))


## v0.1.2 (2026-04-16)

### Refactoring

- **generator**: Improve line unwrapping
  ([`01e516f`](https://github.com/igorrochap/commitgen/commit/01e516f10660fc49364d50223f9e1ff6ef9ad7f0))


## v0.1.1 (2026-04-15)

### Chores

- **cli**: Update default Ollama model
  ([`fe4af0c`](https://github.com/igorrochap/commitgen/commit/fe4af0c5e1cd8b054fe44c68566ab5f2e091daa3))

### Documentation

- **readme**: Update default model
  ([`0084bf3`](https://github.com/igorrochap/commitgen/commit/0084bf3ef8ff34324a2a7623cdd1203dadf9e6f5))

- **readme**: Update installation instructions
  ([`833e882`](https://github.com/igorrochap/commitgen/commit/833e882ff21a06a872f01b456ead8267564f394b))

### Features

- **generator**: Prevent and clean LLM line-wrap artifacts
  ([`48560fe`](https://github.com/igorrochap/commitgen/commit/48560fec6dae75b3255d9bd7cf80a6b57695f6c5))


## v0.1.0 (2026-04-15)

- Initial Release
