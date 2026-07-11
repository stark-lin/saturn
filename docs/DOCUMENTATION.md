# Saturn Documentation References

Saturn keeps implementation directories and their design documents connected through short, reciprocal references. Documentation is written in English and may use the structure that best serves the subject; the policy only standardizes coverage and references.

Every Git-tracked directory has a `README.md` unless it is listed in the central exemption file. Exemptions are restricted to third-party dependencies, generated files, build outputs, caches, test data, migrations, and static-asset-only directories.

Each directory README identifies the directly related documents under `docs/`. Each design document links back to the corresponding directory README files. The `References` section is always the final first- or second-level section, and contains only direct relationships rather than a general link list.

`scripts/check-docs.sh` validates the policy locally and CI runs the same check. It verifies managed-directory README coverage, the final `References` section, relative-link targets, reciprocal README/document links, and the central exemption configuration. It intentionally does not prescribe prose structure, headings other than `References`, document length, or a fixed documentation hierarchy.

## References

- [Project README](../README.md)
- [Documentation Index](README.md)
- [GitHub Automation](../.github/README.md)
- [Workflow Definitions](../.github/workflows/README.md)
- [Development Scripts](../scripts/README.md)
