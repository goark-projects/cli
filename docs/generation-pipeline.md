# Staged code generation

Generation follows a deterministic chain: discover inputs → scan all inputs → validate → bind models → prepare plans → render all outputs → write files.

Scanning, binding and rendering do not write files. A scan, validation, planning or rendering failure prevents the batch from writing outputs or deleting stale files. Writes use per-file atomic replacement, not a multi-file transaction. Goark CLI and its independent ORM tool each own their batch; there is no cross-tool rollback.

`internal/genpipeline` validates stage names and handlers before executing the declared order, stopping on the first error and preserving the wrapped cause. Domain state belongs to the invocation. Add stages after their prerequisites, with explicit inputs and outputs; do not swallow errors.

## Goark CLI

`PrepareAnnotationPlans` scans every selected package before validating and binding annotations. Plans expose `RenderFiles`, which runs extensions in registration order. The project command collects every rendered file before writing.

The extension contract remains `AnnotationDescriptor`, `AnnotationBinder`, and `AnnotationGenerator`. Descriptors define constraints; binders build models after scanning completes; generators consume models without scanning or writing files. Register dependencies before their consumers. Extension names must be unique ignoring case and must not contain path separators.

## ORM CLI

`ormgen.ScanPackages` loads all selected packages, validates annotations, and collects all XML resources before binding entities, DAOs and XML mappings. Each XML path is read once per package; binding consumes the scanned snapshot. Source syntax errors prevent generation, while unresolved generated types remain permissible during initial generation. `ScanPackage` uses the same pipeline. The CLI then plans output paths and calls `genoutput.Prepare` to collect source type information. `Plan.Render` does not read original Go source files.

Responsibilities are separated into `internal/sourcegen` (discovery, AST, annotations, XML), `internal/genmodel` (shared models), `internal/genrender` (same-package rendering), `ormgen/genoutput` (child-package output plans), and `internal/ormcli` (arguments, batch orchestration, file output). The public `ormgen` facade remains compatible.

Entity and DAO package names and layouts are not prescribed. CLI outputs belong to each scanned package's `gen` child directory. With `--config`, put scan options in the configuration file; combining it with `--dir`, `--package`, `--output`, `--build-tag`, or `--type-handler` is rejected.

## Verification boundary

Regression tests cover ordering, invalid stage chains, wrapped errors, later-package failures preventing earlier rendering, and annotation conflicts. Use `--check` and compile generated packages to verify generation without database access. Sessions, transactions and Goark runtime integration require separate validation.

SQL annotations reject unknown attributes, invalid `statementType` values and simultaneous `timeout`/`timeoutDuration`. Web bracket selectors and `param` attributes cannot identify different parameters. Uninitialized Goark generation plans return errors instead of panicking.
