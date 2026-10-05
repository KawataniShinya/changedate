#!/usr/bin/env bash
set -euo pipefail

# Set these variables in a local launcher or in the cron environment.
tool_dir=${CHANGEDATE_TOOL_DIR:?Set CHANGEDATE_TOOL_DIR}
photo_dir=${CHANGEDATE_PHOTO_DIR:?Set CHANGEDATE_PHOTO_DIR}
smb_share=${CHANGEDATE_SMB_SHARE:?Set CHANGEDATE_SMB_SHARE}
smb_root=${CHANGEDATE_SMB_ROOT:?Set CHANGEDATE_SMB_ROOT}
smb_auth_file=${CHANGEDATE_SMB_AUTH_FILE:?Set CHANGEDATE_SMB_AUTH_FILE}
export TZ=${CHANGEDATE_TIMEZONE:-Asia/Tokyo}
lock_file=${CHANGEDATE_LOCK_FILE:-/tmp/changedate-ubuntu.lock}

exec 9>"$lock_file"
if ! flock -n 9; then
  printf '%s: previous changedate Ubuntu run is still active\n' "$(date '+%F %T %Z')" >&2
  exit 0
fi

for path in "$tool_dir" "$photo_dir"; do
  # An automount can report both autofs and cifs for the same target.
  # Restrict both queries to the actual CIFS filesystem.
  if [[ "$(findmnt -n -t cifs -o FSTYPE -T "$path")" != cifs ]] ||
     ! findmnt -n -t cifs -o OPTIONS -T "$path" | grep -Eq '(^|,)rw(,|$)'; then
    printf '%s: NAS is not mounted read-write: %s\n' "$(date '+%F %T %Z')" "$path" >&2
    exit 1
  fi
done

cd "$tool_dir"
log_file="./logs/changedate-ubuntu-$(date +%Y%m%d-%H%M%S).log"
./changedate-linux \
  --dir "$photo_dir" \
  --mode batch \
  --backup-csv-dir ./backup \
  --set-birthtime \
  --smb-share "$smb_share" \
  --smb-root "$smb_root" \
  --smb-auth-file "$smb_auth_file" \
  --timezone "$TZ" \
  --log-file "$log_file" \
  > /dev/null
