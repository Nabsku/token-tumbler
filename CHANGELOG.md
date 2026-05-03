# Changelog

All notable user-facing changes are documented here and in [GitHub Releases](https://github.com/Nabsku/token-tumbler/releases).

This project follows semantic versioning for tagged releases where practical.

## [2.1.0] - 2026-05-03


### Tests

- Add deterministic rotation coverage ([660bc73](https://github.com/Nabsku/token-tumbler/commit/660bc735f263e1990b213fdef45f21b3e031b8bb))
## [2.0.0] - 2026-05-02


### Features

- Redesign token config schema ([13d8a7f](https://github.com/Nabsku/token-tumbler/commit/13d8a7f2897ad8308b3060ce31d14425627208e4))
## [1.1.6] - 2026-05-02


### Bug fixes

- Address publication readiness gaps ([e376d9b](https://github.com/Nabsku/token-tumbler/commit/e376d9b79fe835e5ec80ea83d0f809dda0c87411))


### Documentation

- Refine token permission guidance ([1c26dde](https://github.com/Nabsku/token-tumbler/commit/1c26dde6d35c4daa24390803a80baeb05798fa1b))

- Document token permissions ([188994d](https://github.com/Nabsku/token-tumbler/commit/188994d890dff13ba4b6bb57df408c617447c28b))
## [1.1.5] - 2026-05-02


### Bug fixes

- Always render helm configmap ([76a3956](https://github.com/Nabsku/token-tumbler/commit/76a3956f93cc78e30f757c9d834e598b8b15cd9e))
## [1.1.4] - 2026-05-02


### Bug fixes

- Stabilize helm release deployment test ([5432491](https://github.com/Nabsku/token-tumbler/commit/54324914b9c2259ff1258a37711885f85ed31e8e))


### Continuous integration

- Add helm release diagnostics ([03585d8](https://github.com/Nabsku/token-tumbler/commit/03585d84457ef8a53a75ff80d6e8f0bd6864b929))
## [1.1.2] - 2026-05-01


### Continuous integration

- Test helm release in kind ([d208a56](https://github.com/Nabsku/token-tumbler/commit/d208a560a38b70b229e2bb4a765670a2a125b90e))
## [1.1.1] - 2026-05-01


### Documentation

- Polish public docs ([7ab89d1](https://github.com/Nabsku/token-tumbler/commit/7ab89d11d04d48a4be850f8a8da3128b2bbc8756))
## [1.1.0] - 2026-05-01


### Documentation

- Fix readme mermaid syntax ([f454c84](https://github.com/Nabsku/token-tumbler/commit/f454c844d7db58eec5730e6398d87fcc11110997))

- Automate changelog generation ([b8efcf3](https://github.com/Nabsku/token-tumbler/commit/b8efcf3a099c07f92bc7df5057b220ee993298f8))


### Features

- Add kubernetes leader election ([07754a8](https://github.com/Nabsku/token-tumbler/commit/07754a892e9ff1552a707e3b665dc49dadc65b5a))
## [1.0.2] - 2026-05-01


### Documentation

- Improve public onboarding ([d4f2e0c](https://github.com/Nabsku/token-tumbler/commit/d4f2e0c06078396c2ee9a9bb4d814e49e0a42d98))
## [1.0.1] - 2026-05-01


### Bug fixes

- Restrict helm metrics network policy ([a572d34](https://github.com/Nabsku/token-tumbler/commit/a572d34611a0657462b15400f9c2a47e548fb26f))


### Maintenance

- Prepare repository for public release ([f5528be](https://github.com/Nabsku/token-tumbler/commit/f5528be965143c5940001099b9688c2bd511228e))


### Refactoring

- Split main into internal packages ([4aebec6](https://github.com/Nabsku/token-tumbler/commit/4aebec6387b7368fd1bad630bbb2485d2b458e46))
## [1.0.0] - 2026-04-30


### Bug fixes

- Helm release ([68ad280](https://github.com/Nabsku/token-tumbler/commit/68ad2804522e34fa0a892877a89f4b039442b4ab))

- Remove negation from helmignore ([17a93c0](https://github.com/Nabsku/token-tumbler/commit/17a93c08dbf999a1d3489208bdaf958edb47a446))

- Resolve lint failures ([d524cbb](https://github.com/Nabsku/token-tumbler/commit/d524cbbb5cbccaad75d5139313b89f77a9fadea3))

- Update e2e tests for project API context ([6620950](https://github.com/Nabsku/token-tumbler/commit/6620950767d0b94bf6678e28c6af22ebc67952a8))

- Ran gofmt ([9e06833](https://github.com/Nabsku/token-tumbler/commit/9e068330910f8dc81c67f24f9798d9447a5e576f))

- Do not enable netpols by default ([6c75c98](https://github.com/Nabsku/token-tumbler/commit/6c75c98ea3a1b7e60f9f1ca91ac123c4490ba22e))

- Use Vault KVv2 CAS (check-and-set) to prevent lost updates ([bc53896](https://github.com/Nabsku/token-tumbler/commit/bc53896c2d33c8f662f9f11fadb22982fc75aba3))

- Restore previous token value when metadata write fails after secret write ([5728678](https://github.com/Nabsku/token-tumbler/commit/5728678e22dcd616ab6235bde75a8b8bc808413e))

- Saga-style token rotation with rollback and vault-aware cleanup ([70c7ac2](https://github.com/Nabsku/token-tumbler/commit/70c7ac239dd6d78777bd1fb529677e77cc52535e))

- Enforce secure parent directory checks for file secret store ([a059e0a](https://github.com/Nabsku/token-tumbler/commit/a059e0a3191ce2dfcb4318e7b2adf2c47a4440b6))

- Validate Vault auth credentials preflight and init secret store before token mutation ([3d1d582](https://github.com/Nabsku/token-tumbler/commit/3d1d58202e31d8bec529d325e469a4546159f0a5))

- Update int types to int64 for gitlab.com/gitlab-org/api/client-go v1.46.0 ([77cb372](https://github.com/Nabsku/token-tumbler/commit/77cb372aeffb840b4bc7f30c013a76cbe7f58976))


### Tests

- Add group-path coverage for vault-write-failure rollback ([cc22000](https://github.com/Nabsku/token-tumbler/commit/cc220000acc482cee4e066ae6dd3b2c257b4ace4))
