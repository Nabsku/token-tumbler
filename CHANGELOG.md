# Changelog

All notable user-facing changes are documented here and in [GitHub Releases](https://github.com/Nabsku/token-tumbler/releases).

This project follows semantic versioning for tagged releases where practical.

## [Unreleased]


### Documentation

- Fix readme mermaid syntax ([123d994](https://github.com/Nabsku/token-tumbler/commit/123d994bf914024f7ae2dbd07fd70ca55c39b70e))

- Automate changelog generation ([d6c7a26](https://github.com/Nabsku/token-tumbler/commit/d6c7a2613f418fa82ebab9aeef520432a9e49de8))
## [1.0.2] - 2026-05-01


### Documentation

- Improve public onboarding ([6c6c8fa](https://github.com/Nabsku/token-tumbler/commit/6c6c8fa2b936aff9c174fc83e0e4bf6298ae6602))
## [1.0.1] - 2026-05-01


### Bug fixes

- Restrict helm metrics network policy ([ff320f6](https://github.com/Nabsku/token-tumbler/commit/ff320f6f03a464d45ee6096454d537f4cbfaa13d))


### Maintenance

- Prepare repository for public release ([e5c1afb](https://github.com/Nabsku/token-tumbler/commit/e5c1afb9beb4e4ffa3c8f460c9904501e711a97e))


### Refactoring

- Split main into internal packages ([155c530](https://github.com/Nabsku/token-tumbler/commit/155c530b8a89f50a3e460acde94336a1fbb00978))
## [1.0.0] - 2026-04-30


### Bug fixes

- Helm release ([87d56b4](https://github.com/Nabsku/token-tumbler/commit/87d56b492df2b55ae6d473c2d3cdce32607b26e9))

- Remove negation from helmignore ([9214087](https://github.com/Nabsku/token-tumbler/commit/9214087a7cd1cdb7791b9776d83328d162457e00))

- Resolve lint failures ([a4f0809](https://github.com/Nabsku/token-tumbler/commit/a4f0809d7ef833d712a68f7e8e03f12f0a70ff8f))

- Update e2e tests for project API context ([7c0c91e](https://github.com/Nabsku/token-tumbler/commit/7c0c91e89f33dd3526d3c680062d25205eb4aa2d))

- Ran gofmt ([db921bd](https://github.com/Nabsku/token-tumbler/commit/db921bd1b6882a6dc6d62a4904e24f76cd72f16b))

- Do not enable netpols by default ([425a39a](https://github.com/Nabsku/token-tumbler/commit/425a39af5327d7171e29ff63a97d1b6542854eb4))

- Use Vault KVv2 CAS (check-and-set) to prevent lost updates ([5745cc8](https://github.com/Nabsku/token-tumbler/commit/5745cc8dbbf0a7f5567e595f9c99a6d07220b7ca))

- Restore previous token value when metadata write fails after secret write ([a0fffc0](https://github.com/Nabsku/token-tumbler/commit/a0fffc09ce3443f8bd56eed2a0fe2b5e6ca1de43))

- Saga-style token rotation with rollback and vault-aware cleanup ([ae25579](https://github.com/Nabsku/token-tumbler/commit/ae255796c825c511c1398e078ca580c8112c8cbf))

- Enforce secure parent directory checks for file secret store ([6ef11f1](https://github.com/Nabsku/token-tumbler/commit/6ef11f13ca3262f53bf5f9a2356209d9b1b29c59))

- Validate Vault auth credentials preflight and init secret store before token mutation ([60fa1b3](https://github.com/Nabsku/token-tumbler/commit/60fa1b3339b4eb4f37d6d13838e024fd3e2e169e))

- Update int types to int64 for gitlab.com/gitlab-org/api/client-go v1.46.0 ([dbf375a](https://github.com/Nabsku/token-tumbler/commit/dbf375a3830b9fe209749ead5ab4e8a1bc52eb5e))


### Documentation

- Update llms.txt with expanded project overview ([7826152](https://github.com/Nabsku/token-tumbler/commit/782615274a77a1dfcfc09e59eb7811f806f7cc29))


### Tests

- Add group-path coverage for vault-write-failure rollback ([7275f7f](https://github.com/Nabsku/token-tumbler/commit/7275f7fc426f5ab72bc80436f02561f50604425f))
