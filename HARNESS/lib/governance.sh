#!/usr/bin/env bash

# Repository hygiene and optional specification/approval governance. This module
# expects harness.sh to provide the configuration variables and common helpers.

repo_issue() {
  local label="$1"
  local content="$2"
  [[ -n "$content" ]] || return 0
  if is_true "$HARNESS_STRICT_REPO_HYGIENE"; then
    printf 'error: tracked %s detected:\n%s\n' "$label" "$content" >&2
    return 1
  fi
  printf 'warning: tracked %s detected:\n%s\n' "$label" "$content"
}

repo_doctor() {
  section "Repository doctor"
  if ! command -v git >/dev/null 2>&1 || ! git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    note "skip: repository is not a Git worktree"
    HARNESS_INTERNAL_CHECK_STATUS="skipped"
    HARNESS_INTERNAL_CHECK_DETAIL="repository is not a Git worktree"
    return 0
  fi

  local tracked generated sensitive
  tracked="$(git -C "$ROOT_DIR" ls-files)"
  generated="$(printf '%s\n' "$tracked" | grep -E '(^|/)(node_modules|\.venv|venv|__pycache__|\.pytest_cache|\.mypy_cache|\.ruff_cache|\.next|coverage|test-results)(/|$)|(^|/)\.DS_Store$' || true)"
  sensitive="$(printf '%s\n' "$tracked" | grep -E '(^|/)\.env$|\.(pem|p12|pfx|key)$|(^|/)(id_rsa|id_ed25519|\.npmrc|\.pypirc)$' || true)"

  repo_issue "generated or dependency paths" "$generated"
  repo_issue "secret-like files" "$sensitive"
  note "ok: repository hygiene inspection completed"
}

governance_files() {
  printf '%s\n' \
    "$HARNESS_SPEC_FILE" \
    "$HARNESS_CHANGELOG_FILE" \
    "$HARNESS_PLAN_FILE" \
    "$HARNESS_TEST_PLAN_FILE" \
    "$HARNESS_TRACE_FILE"
}

GOVERNANCE_ACTIVE="false"

governance_doctor_base() {
  GOVERNANCE_ACTIVE="false"
  if [[ "$HARNESS_GOVERNANCE_MODE" == "off" ]]; then
    note "skip: governance checks are disabled"
    HARNESS_INTERNAL_CHECK_STATUS="skipped"
    HARNESS_INTERNAL_CHECK_DETAIL="governance checks are disabled"
    return 0
  fi

  local file present=0 total=0
  while IFS= read -r file; do
    total=$((total + 1))
    [[ -f "$ROOT_DIR/$file" ]] && present=$((present + 1))
  done < <(governance_files)

  if [[ "$HARNESS_GOVERNANCE_MODE" == "auto" && "$present" -eq 0 ]]; then
    note "skip: no governance control files found"
    HARNESS_INTERNAL_CHECK_STATUS="skipped"
    HARNESS_INTERNAL_CHECK_DETAIL="no governance control files found"
    return 0
  fi
  if [[ "$present" -ne "$total" ]]; then
    while IFS= read -r file; do
      [[ -f "$ROOT_DIR/$file" ]] || printf 'missing: %s\n' "$file" >&2
    done < <(governance_files)
    die "governance file set is incomplete"
  fi

  GOVERNANCE_ACTIVE="true"
  local change_id
  change_id="$(active_change_id)"
  while IFS= read -r file; do
    if ! grep -Fq "$change_id" "$ROOT_DIR/$file"; then
      die "$file does not reference active change $change_id"
    fi
  done < <(governance_files)
  note "ok: active change $change_id is traceable across all governance files"
}

active_change_id() {
  local value=""
  if [[ -n "$HARNESS_ACTIVE_CHANGE_ID" ]]; then
    validate_change_id "$HARNESS_ACTIVE_CHANGE_ID"
    printf '%s\n' "$HARNESS_ACTIVE_CHANGE_ID"
    return 0
  fi

  if [[ -n "$HARNESS_ACTIVE_CHANGE_FILE" && -f "$ROOT_DIR/$HARNESS_ACTIVE_CHANGE_FILE" ]]; then
    local active_line_count
    active_line_count="$(awk '!/^[[:space:]]*(#|$)/ { count++ } END { print count+0 }' "$ROOT_DIR/$HARNESS_ACTIVE_CHANGE_FILE")"
    [[ "$active_line_count" -eq 1 ]] || die "$HARNESS_ACTIVE_CHANGE_FILE must contain exactly one active change ID"
    value="$(awk '!/^[[:space:]]*(#|$)/ { gsub(/^[[:space:]]+|[[:space:]]+$/, ""); print }' "$ROOT_DIR/$HARNESS_ACTIVE_CHANGE_FILE")"
    validate_change_id "$value"
    printf '%s\n' "$value"
    return 0
  fi

  local marker_line marker_count
  marker_line="$(grep -Ei '^[[:space:]]*(Active Change|目前變更)[[:space:]]*:' "$ROOT_DIR/$HARNESS_CHANGELOG_FILE" || true)"
  if [[ -n "$marker_line" ]]; then
    marker_count="$(printf '%s\n' "$marker_line" | awk 'END { print NR }')"
    [[ "$marker_count" -eq 1 ]] || die "$HARNESS_CHANGELOG_FILE must contain exactly one structured Active Change marker"
    value="$(grep -Eo "$HARNESS_CHANGE_ID_PATTERN" <<<"$marker_line" | head -n 1 || true)"
    [[ -n "$value" ]] || die "structured Active Change marker does not contain a valid ID"
    validate_change_id "$value"
    printf '%s\n' "$value"
    return 0
  fi

  if is_true "$HARNESS_ALLOW_LEGACY_CHANGE_DISCOVERY"; then
    value="$(grep -Eo "$HARNESS_CHANGE_ID_PATTERN" "$ROOT_DIR/$HARNESS_CHANGELOG_FILE" | head -n 1 || true)"
    [[ -n "$value" ]] || die "could not discover an active change ID in $HARNESS_CHANGELOG_FILE"
    validate_change_id "$value"
    printf '%s\n' "$value"
    return 0
  fi

  die "active change is undefined; set HARNESS_ACTIVE_CHANGE_ID, create $HARNESS_ACTIVE_CHANGE_FILE, or add 'Active Change: <ID>' to $HARNESS_CHANGELOG_FILE"
}

validate_change_id() {
  local value="$1"
  if ! grep -Eq "^(${HARNESS_CHANGE_ID_PATTERN})$" <<<"$value"; then
    die "invalid active change ID: $value"
  fi
}

extract_approval_field() {
  local section_text="$1"
  local label_pattern="$2"
  printf '%s\n' "$section_text" \
    | grep -Ei "^[[:space:]]*-[[:space:]]*(${label_pattern})[[:space:]]*:" \
    | head -n 1 \
    | sed -E 's/^[^:]+:[[:space:]]*//; s/[[:space:]]+$//' \
    || true
}

status_is_complete() {
  grep -Eqi '^(complete|confirmed|approved|完成|已確認|已核准)$' <<<"$1"
}

approval_value_is_filled() {
  [[ -n "$1" ]] && ! grep -Eqi '^(TBD|TODO|PENDING|N/A|NA|None|未定|待確認|待核准)$' <<<"$1"
}

legacy_governance_approval() {
  local change_id="$1"
  local section_text="$2"
  if grep -Eqi '(Gate|關卡)[[:space:]]*4.*(pending|待確認|待核准|未核准)' <<<"$section_text"; then
    die "active change $change_id still marks Gate 4 as pending"
  fi
  if ! grep -Eqi '(Gate|關卡)[[:space:]]*2.*(complete|confirmed|approved|完成|已確認|已核准)' <<<"$section_text"; then
    die "active change $change_id must record completed Gate 2 confirmation"
  fi
  if ! grep -Eqi '(Gate|關卡)[[:space:]]*4.*(complete|approved|完成|已核准)' <<<"$section_text"; then
    die "active change $change_id must record approved Gate 4"
  fi
  if ! grep -Eq '[0-9]{4}-[0-9]{2}-[0-9]{2}' <<<"$section_text"; then
    die "active change $change_id must include an ISO approval date"
  fi
}

extract_change_section() {
  local change_id="$1"
  awk -v change_id="$change_id" '
    $0 ~ "^##[[:space:]]+" change_id "([[:space:]]|$)" {
      active=1
      found=1
    }
    active && found && $0 ~ "^##[[:space:]]+" && $0 !~ "^##[[:space:]]+" change_id "([[:space:]]|$)" {
      exit
    }
    active { print }
  ' "$ROOT_DIR/$HARNESS_PLAN_FILE"
}

governance_approval_base() {
  local change_id section_text gate2 gate4 approved_by approved_on
  change_id="$(active_change_id)"
  section_text="$(extract_change_section "$change_id")"
  [[ -n "$section_text" ]] || die "$HARNESS_PLAN_FILE needs a level-two section beginning with $change_id"

  gate2="$(extract_approval_field "$section_text" 'Gate[[:space:]]*2[[:space:]]+Status|關卡[[:space:]]*2[[:space:]]*狀態')"
  gate4="$(extract_approval_field "$section_text" 'Gate[[:space:]]*4[[:space:]]+Status|關卡[[:space:]]*4[[:space:]]*狀態')"
  approved_by="$(extract_approval_field "$section_text" 'Approved[[:space:]]+By|核准者')"
  approved_on="$(extract_approval_field "$section_text" 'Approved[[:space:]]+On|核准日期')"

  if [[ -z "$gate2$gate4$approved_by$approved_on" ]] && is_true "$HARNESS_ALLOW_LEGACY_APPROVAL_FORMAT"; then
    legacy_governance_approval "$change_id" "$section_text"
    note "ok: active change $change_id has legacy Gate 2 and Gate 4 records"
    return 0
  fi

  status_is_complete "$gate2" || die "active change $change_id must set 'Gate 2 Status' to complete/approved"
  status_is_complete "$gate4" || die "active change $change_id must set 'Gate 4 Status' to complete/approved"
  approval_value_is_filled "$approved_by" || die "active change $change_id must fill 'Approved By'"
  if [[ ! "$approved_on" =~ ^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$ ]]; then
    die "active change $change_id must fill 'Approved On' with an ISO date"
  fi
  note "ok: active change $change_id was approved by $approved_by on $approved_on"
}

governance_doctor() {
  section "Governance doctor"
  governance_doctor_base
  if [[ "$GOVERNANCE_ACTIVE" == "true" ]] && is_true "$HARNESS_REQUIRE_PLAN_APPROVAL"; then
    governance_approval_base
  fi
}

governance_approved() {
  section "Governance approval"
  governance_doctor_base
  if [[ "$GOVERNANCE_ACTIVE" != "true" ]]; then
    die "no active governance record is available for approval validation"
  fi
  governance_approval_base
}
