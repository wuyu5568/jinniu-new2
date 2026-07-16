# 回归 / 演示入口（Windows / 无 make 时使用）
# 用法:
#   .\scripts\regress.ps1 -Test
#   .\scripts\regress.ps1 -Smoke
#   .\scripts\regress.ps1 -SmokeAdmin
#   .\scripts\regress.ps1 -SeedDemo
#   .\scripts\regress.ps1 -All
param(
    [switch]$Test,
    [switch]$Smoke,
    [switch]$SmokeAdmin,
    [switch]$SeedDemo,
    [switch]$All,
    [string]$Base = "http://127.0.0.1:8000"
)

$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

function Assert-Health {
    try {
        $r = Invoke-WebRequest -Uri "$Base/health" -UseBasicParsing -TimeoutSec 3
        if ($r.StatusCode -ne 200) { throw "status $($r.StatusCode)" }
        $j = $r.Content | ConvertFrom-Json
        if ($j.status -ne "ok" -or $j.db -ne "ok") {
            throw "health body status=$($j.status) db=$($j.db)"
        }
    } catch {
        Write-Error "服务未就绪: $Base/health — 请先启动后端 (go run / bin/app.exe)；$_"
    }
}

if ($All) {
    $Test = $true
    $Smoke = $true
    $SmokeAdmin = $true
}

if (-not ($Test -or $Smoke -or $SmokeAdmin -or $SeedDemo)) {
    Write-Host "用法: .\scripts\regress.ps1 -Test | -Smoke | -SmokeAdmin | -SeedDemo | -All"
    exit 1
}

if ($Test) {
    Write-Host "==> go test biz"
    go test ./app/app/internal/biz/... -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($SeedDemo) {
    Write-Host "==> seed_demo_users"
    go run scripts/seed_demo_users.go
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($Smoke) {
    Assert-Health
    Write-Host "==> smoke_mainpath"
    go run scripts/smoke_mainpath.go
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($SmokeAdmin) {
    Assert-Health
    Write-Host "==> smoke_admin_write"
    go run scripts/smoke_admin_write.go
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "OK"
