# This script validates Saturn documentation coverage and reciprocal references.
set -eu

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root"

exemptions_file=.saturn-docs-exemptions

if [ ! -f "$exemptions_file" ]; then
  printf 'Missing documentation exemptions file: %s\n' "$exemptions_file" >&2
  exit 1
fi

is_exempt_directory() {
  directory=$1

  while IFS= read -r pattern || [ -n "$pattern" ]; do
    case "$pattern" in
      ''|'#'*) continue ;;
    esac
    case "$directory" in
      $pattern) return 0 ;;
    esac
  done < "$exemptions_file"

  return 1
}

references_content() {
  awk '
    /^# References[[:space:]]*$/ || /^## References[[:space:]]*$/ { in_references = 1; next }
    in_references { print }
  ' "$1"
}

reference_targets() {
  references_content "$1" | sed -nE 's@.*\]\(([^)#]+)(#[^)]*)?\).*@\1@p'
}

resolve_reference() {
  source_file=$1
  reference=$2
  reference=${reference%%#*}

  case "$reference" in
    ''|http://*|https://*|mailto:*) return 1 ;;
  esac

  source_directory=$(dirname "$source_file")
  candidate=$source_directory/$reference

  if [ ! -e "$candidate" ]; then
    return 1
  fi

  candidate_directory=$(dirname "$candidate")
  candidate_name=$(basename "$candidate")
  printf '%s/%s\n' "$(cd "$candidate_directory" && pwd)" "$candidate_name"
}

contains_resolved_reference() {
  source_file=$1
  expected_target=$2

  while IFS= read -r reference || [ -n "$reference" ]; do
    resolved=$(resolve_reference "$source_file" "$reference" || true)
    if [ "$resolved" = "$expected_target" ]; then
      return 0
    fi
  done <<EOF
$(reference_targets "$source_file")
EOF

  return 1
}

check_final_references_section() {
  file=$1

  if ! rg -q '^(#|##) References[[:space:]]*$' "$file"; then
    printf 'Missing References section: %s\n' "$file" >&2
    return 1
  fi

  last_heading=$(rg '^(#|##)[[:space:]]' "$file" | tail -n 1 || true)
  if [ "$last_heading" != '## References' ] && [ "$last_heading" != '# References' ]; then
    printf 'References must be the final first- or second-level section: %s\n' "$file" >&2
    return 1
  fi

  if ! references_content "$file" | awk '
    NF && $0 !~ /^- \[[^]]+\]\([^)]+\)$/ { exit 1 }
  '; then
    printf 'References may contain only Markdown reference-list items: %s\n' "$file" >&2
    return 1
  fi
}

check_reference_targets() {
  file=$1

  while IFS= read -r reference || [ -n "$reference" ]; do
    case "$reference" in
      http://*|https://*|mailto:*)
        printf 'External reference is not allowed in References: %s -> %s\n' "$file" "$reference" >&2
        return 1
        ;;
    esac
    if ! resolve_reference "$file" "$reference" >/dev/null; then
      printf 'Invalid relative reference: %s -> %s\n' "$file" "$reference" >&2
      return 1
    fi
  done <<EOF
$(reference_targets "$file")
EOF
}

managed_directories=$(mktemp)
trap 'rm -f "$managed_directories"' EXIT

git ls-files --cached --others --exclude-standard | awk -F/ '
  NF > 1 {
    directory = $1
    print directory
    for (part = 2; part < NF; part++) {
      directory = directory "/" $part
      print directory
    }
  }
' | sort -u > "$managed_directories"

status=0

if [ ! -f README.md ]; then
  printf 'Missing README for repository root\n' >&2
  status=1
fi

while IFS= read -r directory || [ -n "$directory" ]; do
  if is_exempt_directory "$directory"; then
    continue
  fi
  if [ ! -f "$directory/README.md" ]; then
    printf 'Missing README for managed directory: %s\n' "$directory" >&2
    status=1
  fi
done < "$managed_directories"

documentation_files=$(git ls-files --cached --others --exclude-standard -- '*.md')

for file in $documentation_files; do
  case "$file" in
    README.md|*/README.md|docs/*.md|docs/*/*.md)
      check_final_references_section "$file" || status=1
      check_reference_targets "$file" || status=1
      ;;
  esac
done

for document in $(git ls-files --cached --others --exclude-standard -- 'docs/*.md' 'docs/*/*.md'); do
  case "$document" in
    */README.md) continue ;;
  esac

  has_code_directory_reference=false
  while IFS= read -r reference || [ -n "$reference" ]; do
    resolved=$(resolve_reference "$document" "$reference" || true)
    case "$resolved" in
      "$repository_root"/docs/*) ;;
      "$repository_root"/README.md|"$repository_root"/*/README.md)
        has_code_directory_reference=true
        ;;
    esac
  done <<EOF
$(reference_targets "$document")
EOF

  if [ "$has_code_directory_reference" != true ]; then
    printf 'Design document must reference a related code-directory README: %s\n' "$document" >&2
    status=1
  fi
done

for readme in $(git ls-files --cached --others --exclude-standard -- 'README.md' '*/README.md'); do
  has_design_reference=false
  while IFS= read -r reference || [ -n "$reference" ]; do
    resolved=$(resolve_reference "$readme" "$reference" || true)
    case "$resolved" in
      "$repository_root"/docs/*.md)
        has_design_reference=true
        if ! contains_resolved_reference "${resolved#"$repository_root"/}" "$repository_root/$readme"; then
          printf 'Missing reciprocal reference: %s <-> %s\n' "$readme" "${resolved#"$repository_root"/}" >&2
          status=1
        fi
        ;;
    esac
  done <<EOF
$(reference_targets "$readme")
EOF

  if [ "$has_design_reference" != true ]; then
    printf 'README must reference a related document under docs/: %s\n' "$readme" >&2
    status=1
  fi
done

for document in $(git ls-files --cached --others --exclude-standard -- 'docs/*.md' 'docs/*/*.md'); do
  while IFS= read -r reference || [ -n "$reference" ]; do
    resolved=$(resolve_reference "$document" "$reference" || true)
    case "$resolved" in
      "$repository_root"/*/README.md|"$repository_root"/README.md)
        if ! contains_resolved_reference "${resolved#"$repository_root"/}" "$repository_root/$document"; then
          printf 'Missing reciprocal reference: %s <-> %s\n' "$document" "${resolved#"$repository_root"/}" >&2
          status=1
        fi
        ;;
    esac
  done <<EOF
$(reference_targets "$document")
EOF
done

exit "$status"
