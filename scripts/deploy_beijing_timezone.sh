#!/usr/bin/env bash
set -euo pipefail

CFG=/etc/jinniu/config.yaml
MIGRATE_SQL="${HOME}/migrate_beijing_wall_clock.sql"

echo "== stop jinniu =="
sudo systemctl stop jinniu.service || true
sleep 1

echo "== parse DSN =="
eval "$(python3 <<'PY'
import re, shlex
text = open("/etc/jinniu/config.yaml", encoding="utf-8").read()
m = re.search(r"source:\s*[\"']?([^\"'\n]+)", text)
src = m.group(1).strip()
m = re.match(r"([^:]+):([^@]*)@tcp\(([^)]+)\)/([^?]+)(\?.*)?", src)
user, pwd, hostport, db, query = m.groups()
db = db.split("?")[0]
host, port = (hostport.split(":") + ["3306"])[:2]
print(f"DB_USER={shlex.quote(user)}")
print(f"DB_PASS={shlex.quote(pwd)}")
print(f"DB_HOST={shlex.quote(host)}")
print(f"DB_PORT={shlex.quote(port)}")
print(f"DB_NAME={shlex.quote(db)}")
print(f"DSN_QUERY={shlex.quote(query or '')}")
PY
)"

mysql_cmd() {
  mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" "$@"
}

echo "== check migration marker =="
ALREADY=$(mysql_cmd -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='schema_migrations';")
if [[ "$ALREADY" == "1" ]]; then
  DONE=$(mysql_cmd -N -e "SELECT COUNT(*) FROM schema_migrations WHERE id='beijing_wall_clock_v1';")
else
  DONE=0
fi

if [[ "$DONE" == "1" ]]; then
  echo "beijing_wall_clock_v1 already applied — skip DATETIME +8h"
else
  echo "== apply DATETIME +8h =="
  # sample before
  mysql_cmd -e "SELECT id, created_at FROM ledger_entries ORDER BY id DESC LIMIT 1;"
  mysql_cmd < "$MIGRATE_SQL"
  mysql_cmd -e "SELECT id, created_at FROM ledger_entries ORDER BY id DESC LIMIT 1;"
  echo "DATETIME migration OK"
fi

echo "== OS timezone Asia/Shanghai =="
sudo timedatectl set-timezone Asia/Shanghai
timedatectl | head -n 6
date

echo "== update app DSN loc/time_zone =="
sudo python3 <<'PY'
import re
path = "/etc/jinniu/config.yaml"
with open(path, encoding="utf-8") as f:
    text = f.read()

def fix_source(src: str) -> str:
    # strip existing loc / time_zone query params, then append Beijing ones
    if "?" in src:
        base, q = src.split("?", 1)
        parts = [p for p in q.split("&") if p and not p.startswith("loc=") and not p.startswith("time_zone=")]
        q = "&".join(parts)
        src = base + (("?" + q) if q else "")
    sep = "&" if "?" in src else "?"
    if "parseTime=" not in src:
        src = src + sep + "parseTime=True"
        sep = "&"
    src = src + sep + "loc=Asia%2FShanghai&time_zone=%27%2B08%3A00%27"
    return src

def repl(m):
    prefix, quote, src = m.group(1), m.group(2), m.group(3)
    # quote may be empty
    new = fix_source(src.strip())
    if quote:
        return f"{prefix}{quote}{new}{quote}"
    return f"{prefix}{new}"

new_text, n = re.subn(
    r"(source:\s*)([\"']?)([^\"'\n]+)\2",
    repl,
    text,
    count=1,
)
if n != 1:
    raise SystemExit(f"failed to patch source (matches={n})")
with open(path, "w", encoding="utf-8") as f:
    f.write(new_text)
print("config source patched (loc=Asia/Shanghai, time_zone=+08:00)")
PY

echo "== start jinniu =="
sudo systemctl start jinniu.service
sleep 2
sudo systemctl is-active jinniu.service
curl -sS http://127.0.0.1:8000/health
echo

echo "== verify MySQL session / sample =="
mysql_cmd -e "SELECT @@session.time_zone AS session_tz, NOW() AS mysql_now; SELECT id, created_at FROM ledger_entries ORDER BY id DESC LIMIT 3;"

echo "== done =="
