# Deterministic inventory before source investigation

```sh
noodle scan /path/to/project /path/to/related-repository -o inventory.yaml
```

`noodle scan` is local, read-only and independent of an LLM. It prints a compact summary
and writes `noodle.inventory/v1alpha1` YAML. Existing Model/View/Fragment contracts are
unchanged. This report is not a curated model or a Fragment; do not put it in `discovered/`.

## What it extracts

| Input | Deterministic result |
|---|---|
| Standalone/rendered Kubernetes YAML/JSON | Existing Kubernetes adapter's elements, declared ports, connections, references and unresolved source locations |
| Compose files | Services, literal images, container-side TCP/UDP ports, `depends_on` references |
| package.json | Package/dependency names and dependency scopes; no scripts, version URLs or credentials |
| requirements*.txt / requirements*.in | Dependency names; includes/directives remain unresolved and are never followed |
| Helm/Kustomize inputs | Inventory and `render-required`; never execute or infer deployed resources from raw templates |
| Terraform, pyproject/Pipfile, Go/Rust/JVM/PHP manifests, Dockerfiles and CI | Inventory, fingerprints and explicit parser gaps |

Compose dependencies and library declarations are not runtime connections. Kubernetes
manifest directories are separate extraction scopes, not proven environments. Cross-folder
deployment context and unknown resource/operator semantics still need interpretation.

## Efficient use

1. Save the inventory; use its summary and relevant sections rather than dumping it all.
2. Resolve `src.file` through `roots`: the prefix is a root ID, followed by a relative
   file path. Every extracted item/dependency has an anchor line.
3. Compare `files` paths/SHA-256 with the previous report. Reuse unchanged facts; examine
   added/changed/removed inputs and unresolved questions that matter to the requested view.
4. Curate those facts into the shared v1alpha2 model and its evidence map. Do not turn
   inventory file entries or every library into diagram nodes.
5. Render project templates only when needed and authorized, then use existing
   `noodle discover k8s` on rendered output. The scanner itself never renders/fetches it.

No persistent extraction cache is implemented: hashes make changes explicit; the scan
itself is cheap local work. Token estimates are byte-based approximations, not measured
LLM savings. Dynamic values remain unresolved; process environment variables are not read.

## Boundaries

Credential/state/variable files, symlinks and common dependency/generated directories are
excluded. Built-in directory exclusions are not a complete .gitignore interpreter.
Files over 8 MiB are reported unresolved. Dependency versions, scripts, environment values
and Secret payloads are not copied to the report. Scanning proves declarations, not live
listeners, reachability, security enforcement or deployment health.
