#!/usr/bin/env bash
set -euo pipefail

echo "== stop service =="
sudo systemctl stop jinniu.service || true
sleep 1

echo "== migrate last_settled_rate =="
python3 <<'PY'
import os, re, subprocess, sys

cfg = "/etc/jinniu/config.yaml"
with open(cfg, "r", encoding="utf-8") as f:
    text = f.read()
m = re.search(r"source:\s*[\"']?([^\"'\n]+)", text)
if not m:
    sys.exit("no data.database.source in config")
src = m.group(1).strip()
m = re.match(r"([^:]+):([^@]*)@tcp\(([^)]+)\)/([^?]+)", src)
if not m:
    sys.exit("cannot parse DSN")
user, pwd, hostport, db = m.groups()
db = db.split("?")[0]
host, port = (hostport.split(":") + ["3306"])[:2]

def mysql_cmd(*args, input_sql=None):
    cmd = ["mysql", "-h", host, "-P", port, "-u", user, f"-p{pwd}", db, *args]
    return subprocess.run(cmd, input=input_sql, text=True, capture_output=True)

check = mysql_cmd(
    "-N",
    "-e",
    "SELECT COUNT(*) FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='locations' "
    "AND COLUMN_NAME='last_settled_rate';",
)
if check.returncode != 0:
    print(check.stderr, file=sys.stderr)
    sys.exit(check.returncode)
if check.stdout.strip() == "1":
    print("column last_settled_rate already exists — skip ALTER")
else:
    sql_path = os.path.expanduser("~/migrate_last_settled_rate.sql")
    with open(sql_path, "r", encoding="utf-8") as f:
        sql = f.read()
    r = mysql_cmd(input_sql=sql)
    if r.returncode != 0:
        print(r.stderr, file=sys.stderr)
        sys.exit(r.returncode)
    print("ALTER applied OK")

v = mysql_cmd("-N", "-e", "SHOW COLUMNS FROM locations LIKE 'last_settled_rate';")
if "last_settled_rate" not in v.stdout:
    print(v.stderr, file=sys.stderr)
    sys.exit("verify failed: column missing")
print("verify: last_settled_rate present")
PY

echo "== deploy binary =="
chmod +x "$HOME/app.new"
if [[ -f "$HOME/app" ]]; then
  mv "$HOME/app" "$HOME/app.bak.$(date +%Y%m%d%H%M%S)"
fi
mv "$HOME/app.new" "$HOME/app"
chmod +x "$HOME/app"
echo "ExecStart=$(systemctl show -p ExecStart --value jinniu.service || true)"

sudo systemctl start jinniu.service
sleep 2
sudo systemctl is-active jinniu.service
curl -sS http://127.0.0.1:8000/health
echo
echo "== done =="
