# Architecture Decision Records

One decision per file, numbered, never rewritten. A reversed decision gets a new ADR
that supersedes the old one.

| ADR | Title | Status |
|---|---|---|
| [0001](0001-own-stack.md) | Build our own stack instead of adopting LikeC4 | Accepted |
| [0002](0002-model-view-layout-split.md) | Separate model, views and layout | Accepted |
| [0003](0003-connection-semantics.md) | An arrow is a connection | Accepted |
| [0004](0004-topology-vs-sequence.md) | Topology and sequence are separate views | Accepted |
| [0005](0005-git-source-of-truth.md) | Git is the source of truth; artifacts are generated | Accepted |
| [0006](0006-house-style.md) | House style enforced by lint | Accepted |
| [0007](0007-contract-files-and-references.md) | Contract files and how they reference each other | Accepted |
| [0008](0008-node-status-and-multiplicity.md) | Node status and multiplicity are part of the model | Accepted |
| [0009](0009-view-type-field.md) | A view's perspective is `type`, not `kind` | Accepted |
| [0010](0010-denied-and-enforcement.md) | A denied connection names what enforces it | Accepted |
| [0011](0011-sequence-messages.md) | A sequence step is a message over a model connection | Accepted |
| [0012](0012-landscape-view.md) | A landscape view shows the technology stack | Accepted |
| [0013](0013-offerings-and-catalog-view.md) | Platform offerings live in the model; a catalog view shows them | Accepted |
| [0014](0014-planned-target.md) | A planned element names when it is expected | Accepted |
| [0015](0015-discovered-fragments.md) | Adapters write fragments; the curated model matches them | Accepted |
| [0016](0016-discovery-before-viewer.md) | Static Kubernetes discovery comes before the viewer | Accepted |
| [0017](0017-unresolved-and-resolutions.md) | The k8s adapter decodes with yaml.v3 and reports what it cannot resolve | Accepted |
| [0018](0018-credential-implied-connections-and-operator-rules.md) | Credential-implied connections and operator rules | Accepted |
| [0019](0019-access-grants-and-matrix.md) | Access grants live in the model; a matrix view shows them | Accepted |
| [0020](0020-access-matrix-before-graph.md) | The access matrix is built now, on its own traversal | Accepted |
| [0021](0021-project-architecture-and-mermaid.md) | A source-backed architecture model generates Mermaid | Accepted |
| [0022](0022-deterministic-repository-inventory.md) | Inventory before architecture interpretation | Accepted |

Template: Context · Decision · Consequences. Keep it under a page.
